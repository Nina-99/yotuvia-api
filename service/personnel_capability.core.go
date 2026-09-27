package service

// This file holds the single definition of the personnel↔service capability
// rule shared by the booking WRITE validation and the picker READ filter.
//
// The rule (deliberately backward compatible):
//
//   - a professional with NO capability rows is UNRESTRICTED and may be booked
//     for any of the centre's services;
//   - a professional WITH capability rows may be booked ONLY for services among
//     them.
//
// The booking path evaluates it in Go via PersonnelCoversServices; the picker
// filter mirrors it in SQL (repository.PersonnelRepositoryImpl.FindAll). The
// agreement test pins the two together so a professional the picker offers is
// exactly one the booking accepts.
//
// It lives in the service package (not a dedicated one) on purpose: a separate
// package would push `go list ./...` past the project's 15-package invariant.

// PersonnelCoversServices reports whether a professional may be booked for the
// requested services under the capability rule above. An empty capability set
// means "unrestricted" and always returns true.
func PersonnelCoversServices(capabilityServiceIDs, requestedServiceIDs []uint) bool {
	if len(capabilityServiceIDs) == 0 {
		return true
	}
	covered := make(map[uint]struct{}, len(capabilityServiceIDs))
	for _, id := range capabilityServiceIDs {
		covered[id] = struct{}{}
	}
	for _, id := range requestedServiceIDs {
		if _, ok := covered[id]; !ok {
			return false
		}
	}
	return true
}
