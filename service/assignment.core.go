package service

import (
	"github.com/Nina-99/yotuvia-api/apperrors"
	"github.com/Nina-99/yotuvia-api/data/request"
)

// This file holds the single definition of "a multi-professional assignment
// plan is valid" shared by the booking WRITE path (POST /reservation/) and the
// availability chain READ path (GET /availability/search). Keeping one
// predicate means the search can never accept a plan the booking would reject
// (or vice versa).
//
// It lives in the service package (not a dedicated one) on purpose: a separate
// package would push `go list ./...` past the project's 15-package invariant.

// BuildAssignmentPlan validates an OPTIONAL assignments plan against the
// requested service ids and returns the serviceID→personnelID map.
//
// DOCUMENTED RULES:
//
//   - the plan must cover service_ids EXACTLY once: missing, duplicate or
//     unknown service → apperrors.ErrBadRequest;
//   - every personnel_id must be non-zero → otherwise ErrBadRequest.
//
// The caller keeps service_ids order: the derived lead is the professional of
// service_ids[0] (plan[serviceIDs[0]]), and any per-line build iterates the
// request order so the derived segments line up with what the client saw.
func BuildAssignmentPlan(serviceIDs []uint, assignments []request.ServiceAssignment) (map[uint]uint, error) {
	// Equal length plus "no duplicate service" plus "every requested service
	// present" is a bijection; each check is spelled out for a precise 400.
	if len(assignments) != len(serviceIDs) {
		return nil, apperrors.ErrBadRequest
	}
	requested := make(map[uint]struct{}, len(serviceIDs))
	for _, id := range serviceIDs {
		requested[id] = struct{}{}
	}
	byService := make(map[uint]uint, len(assignments))
	for _, a := range assignments {
		if a.ServiceID == 0 || a.PersonnelID == 0 {
			return nil, apperrors.ErrBadRequest
		}
		if _, dup := byService[a.ServiceID]; dup {
			return nil, apperrors.ErrBadRequest // duplicate service in the plan
		}
		if _, ok := requested[a.ServiceID]; !ok {
			return nil, apperrors.ErrBadRequest // service not requested
		}
		byService[a.ServiceID] = a.PersonnelID
	}
	for _, id := range serviceIDs {
		if _, ok := byService[id]; !ok {
			return nil, apperrors.ErrBadRequest // requested service not covered
		}
	}
	return byService, nil
}
