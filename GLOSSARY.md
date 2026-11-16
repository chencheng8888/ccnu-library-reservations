# CCNU Library Reservations

A Go client for reserving seats in CCNU library spaces.

## Language

### Places

**Building**:
A library building that contains one or more Floors.
_Avoid_: Venue, library, branch

**Floor**:
A reservable level within a Building.
_Avoid_: Level, area

**Space**:
A reservable area on a Floor that contains Seats, such as 南湖分馆一楼中庭开敞座位区.
_Avoid_: Room, zone, area

**Seat**:
A single reservable place within a Space, identified by a stable Seat ID and a human-readable label.
_Avoid_: Device, position

**Space Code**:
A short alias for a well-known Space, used instead of its upstream ID.
_Avoid_: Room code, room key

### Reservations

**Reservation**:
A student's claim on a Seat for a specific date and time window.
_Avoid_: Booking, order, make

**Reservation Window**:
The date and minute-of-day range requested or held by a Reservation.
_Avoid_: Period, slot, time range

**Free Seat**:
A Seat that is free for the entire requested Reservation Window.
_Avoid_: Available seat, empty seat

**Occupancy**:
A time range in which a Seat cannot be reserved, whether due to another Reservation or an upstream block.
_Avoid_: Busy period, mark

### Booking Rules

**Step**:
The smallest time increment in which a Reservation Window can be expressed.
_Avoid_: Interval, slot

**Max Duration**:
The longest Reservation Window a student may hold in one Reservation.
_Avoid_: Limit, max minute
