package availability

import "time"

// AvailabilityRule is a recurring weekly schedule entry for a branch.
type AvailabilityRule struct {
	ID        string
	BranchID  string
	DayOfWeek int
	StartTime time.Time
	EndTime   time.Time
	IsActive  bool
	CreatedAt time.Time
}

// DateOverride is a one-off exception to the weekly availability rules for a
// specific calendar date.
type DateOverride struct {
	ID        string
	BranchID  string
	Date      time.Time
	IsClosed  bool
	OpenTime  *time.Time
	CloseTime *time.Time
	Note      string
	CreatedAt time.Time
}

// Slot is a computed, bookable time window for a service on a given day.
// Slots are not stored in the database — they are generated at query time.
type Slot struct {
	StartTime         time.Time
	EndTime           time.Time
	RemainingCapacity int
}

// BookingSlot is a simplified representation of an active booking used for
// capacity and conflict calculations.
type BookingSlot struct {
	StartTime time.Time
	EndTime   time.Time
	Quantity  int
}

// LockSlot is a simplified representation of an active booking lock used for
// capacity and conflict calculations.
type LockSlot struct {
	StartTime time.Time
	EndTime   time.Time
	Quantity  int
}

// GetSlotsRequest is the HTTP request DTO for the GET /availability endpoint.
type GetSlotsRequest struct {
	BranchID  string `form:"branch_id" binding:"required"`
	ServiceID string `form:"service_id" binding:"required"`
	Date      string `form:"date" binding:"required"`
}

// SlotResponse is the HTTP response DTO for a single slot.
type SlotResponse struct {
	StartTime         string `json:"start_time"`
	EndTime           string `json:"end_time"`
	RemainingCapacity int    `json:"remaining_capacity"`
}
