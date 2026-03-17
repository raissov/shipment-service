package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/raissov/shipment-service/gen/shipment/v1"
	"github.com/raissov/shipment-service/internal/application"
	"github.com/raissov/shipment-service/internal/domain"
)

// Handler implements the gRPC ShipmentService.
type Handler struct {
	pb.UnimplementedShipmentServiceServer
	service *application.ShipmentService
}

// NewHandler creates a new gRPC handler.
func NewHandler(service *application.ShipmentService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateShipment(ctx context.Context, req *pb.CreateShipmentRequest) (*pb.CreateShipmentResponse, error) {
	if req.GetReferenceNumber() == "" {
		return nil, status.Error(codes.InvalidArgument, "reference_number is required")
	}
	if req.GetOrigin() == "" {
		return nil, status.Error(codes.InvalidArgument, "origin is required")
	}
	if req.GetDestination() == "" {
		return nil, status.Error(codes.InvalidArgument, "destination is required")
	}

	input := application.CreateShipmentInput{
		ReferenceNumber:     req.GetReferenceNumber(),
		Origin:              req.GetOrigin(),
		Destination:         req.GetDestination(),
		ShipmentAmountCents: req.GetShipmentAmountCents(),
		DriverRevenueCents:  req.GetDriverRevenueCents(),
	}
	if req.GetDriver() != nil {
		input.DriverName = req.GetDriver().GetName()
		input.DriverPhone = req.GetDriver().GetPhone()
	}
	if req.GetUnit() != nil {
		input.UnitNumber = req.GetUnit().GetUnitNumber()
		input.UnitType = req.GetUnit().GetUnitType()
	}

	shipment, err := h.service.CreateShipment(ctx, input)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create shipment: %v", err)
	}

	return &pb.CreateShipmentResponse{
		Shipment: shipmentToProto(shipment),
	}, nil
}

func (h *Handler) GetShipment(ctx context.Context, req *pb.GetShipmentRequest) (*pb.GetShipmentResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	shipment, err := h.service.GetShipment(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Error(codes.NotFound, "shipment not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get shipment: %v", err)
	}

	return &pb.GetShipmentResponse{
		Shipment: shipmentToProto(shipment),
	}, nil
}

func (h *Handler) ListShipments(ctx context.Context, req *pb.ListShipmentsRequest) (*pb.ListShipmentsResponse, error) {
	shipments, total, err := h.service.ListShipments(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list shipments: %v", err)
	}

	pbShipments := make([]*pb.Shipment, len(shipments))
	for i, s := range shipments {
		pbShipments[i] = shipmentToProto(s)
	}

	return &pb.ListShipmentsResponse{
		Shipments: pbShipments,
		Total:     int32(total),
	}, nil
}

func (h *Handler) AddStatusEvent(ctx context.Context, req *pb.AddStatusEventRequest) (*pb.AddStatusEventResponse, error) {
	if req.GetShipmentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "shipment_id is required")
	}
	if req.GetStatus() == pb.ShipmentStatus_SHIPMENT_STATUS_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "status is required")
	}

	domainStatus := protoStatusToDomain(req.GetStatus())

	event, shipment, err := h.service.AddStatusEvent(ctx, req.GetShipmentId(), domainStatus, req.GetComment())
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Error(codes.NotFound, "shipment not found")
		}
		if errors.Is(err, domain.ErrInvalidTransition) || errors.Is(err, domain.ErrInvalidStatus) || errors.Is(err, domain.ErrDuplicateStatus) {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "failed to add status event: %v", err)
	}

	return &pb.AddStatusEventResponse{
		Event:    eventToProto(event),
		Shipment: shipmentToProto(shipment),
	}, nil
}

func (h *Handler) GetShipmentHistory(ctx context.Context, req *pb.GetShipmentHistoryRequest) (*pb.GetShipmentHistoryResponse, error) {
	if req.GetShipmentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "shipment_id is required")
	}

	events, err := h.service.GetShipmentHistory(ctx, req.GetShipmentId())
	if err != nil {
		if errors.Is(err, domain.ErrShipmentNotFound) {
			return nil, status.Error(codes.NotFound, "shipment not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get history: %v", err)
	}

	pbEvents := make([]*pb.StatusEvent, len(events))
	for i, e := range events {
		pbEvents[i] = eventToProto(e)
	}

	return &pb.GetShipmentHistoryResponse{
		Events: pbEvents,
	}, nil
}

// --- Mapping helpers ---

var domainToProtoStatus = map[domain.Status]pb.ShipmentStatus{
	domain.StatusPending:   pb.ShipmentStatus_SHIPMENT_STATUS_PENDING,
	domain.StatusPickedUp:  pb.ShipmentStatus_SHIPMENT_STATUS_PICKED_UP,
	domain.StatusInTransit: pb.ShipmentStatus_SHIPMENT_STATUS_IN_TRANSIT,
	domain.StatusDelivered: pb.ShipmentStatus_SHIPMENT_STATUS_DELIVERED,
	domain.StatusCancelled: pb.ShipmentStatus_SHIPMENT_STATUS_CANCELLED,
}

var protoToDomainStatus = map[pb.ShipmentStatus]domain.Status{
	pb.ShipmentStatus_SHIPMENT_STATUS_PENDING:    domain.StatusPending,
	pb.ShipmentStatus_SHIPMENT_STATUS_PICKED_UP:  domain.StatusPickedUp,
	pb.ShipmentStatus_SHIPMENT_STATUS_IN_TRANSIT: domain.StatusInTransit,
	pb.ShipmentStatus_SHIPMENT_STATUS_DELIVERED:  domain.StatusDelivered,
	pb.ShipmentStatus_SHIPMENT_STATUS_CANCELLED:  domain.StatusCancelled,
}

func shipmentToProto(s *domain.Shipment) *pb.Shipment {
	return &pb.Shipment{
		Id:              s.ID,
		ReferenceNumber: s.ReferenceNumber,
		Origin:          s.Origin,
		Destination:     s.Destination,
		Status:          domainToProtoStatus[s.Status],
		Driver: &pb.DriverDetails{
			Name:  s.Driver.Name,
			Phone: s.Driver.Phone,
		},
		Unit: &pb.UnitDetails{
			UnitNumber: s.Unit.UnitNumber,
			UnitType:   s.Unit.UnitType,
		},
		ShipmentAmountCents: s.ShipmentAmountCents,
		DriverRevenueCents:  s.DriverRevenueCents,
		CreatedAt:           timestamppb.New(s.CreatedAt),
		UpdatedAt:           timestamppb.New(s.UpdatedAt),
	}
}

func eventToProto(e *domain.StatusEvent) *pb.StatusEvent {
	return &pb.StatusEvent{
		Id:         e.ID,
		ShipmentId: e.ShipmentID,
		Status:     domainToProtoStatus[e.Status],
		Comment:    e.Comment,
		OccurredAt: timestamppb.New(e.OccurredAt),
	}
}

func protoStatusToDomain(s pb.ShipmentStatus) domain.Status {
	if ds, ok := protoToDomainStatus[s]; ok {
		return ds
	}
	return domain.Status("unknown")
}
