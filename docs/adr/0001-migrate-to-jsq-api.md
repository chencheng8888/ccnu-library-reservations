# Migrate to the new /jsq reservation API with breaking renames

The upstream reservation system is now served over HTTPS at `kjyy.ccnu.edu.cn/rem` and `/jsq`; the old `/clientweb` endpoints return 403 and the old client cannot work. We will rewrite the library against the new API and align its language with the domain: `Authenticator`/`Reserver`, `Space` instead of Room, `SessionToken` instead of a cookie, and whole-window Free Seats instead of computed occupancy intervals. This is a breaking change: the project is untagged, and the old names describe a system that no longer exists.

## Considered Options

- Keep the old `Auther`/`Reverser`/`GetCookie` names: rejected because `GetCookie` and `Room` would be actively misleading.
- Add a new `Client` and keep the old interfaces as deprecated wrappers: rejected to avoid maintaining two public surfaces.

## Consequences

- Callers migrate from `NewAuther`/`NewReverser`/`Rooms` to the new API and Space IDs.
- The transport must handle CAS redirects, the `/rem` handoff token exchange, and per-request HMAC signing.
- `Seat.IsFree`/`OccupyStates` are removed; partial availability is out of scope for now.
