package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/apsv/goal-tracker/backend/internal/auth"
	"github.com/apsv/goal-tracker/backend/internal/models"
	"github.com/google/uuid"
)

// debugReportMaxBodyBytes caps the request body at 256 KB. A ring buffer of
// 500 breadcrumbs at ~512 B each is roughly 256 KB; we want headroom for the
// description and state block without letting clients stream MBs of junk.
const debugReportMaxBodyBytes = 256 * 1024

// debugReportMaxDescriptionChars caps the user-supplied description server-side.
// Frontend caps input at 2000 chars; 4096 gives headroom while keeping the
// body cap from being the only defence against stuffed descriptions.
const debugReportMaxDescriptionChars = 4096

// createDebugReport handles POST /api/v1/debug-reports.
// Stores a diagnostic report captured on-device (either via shake gesture or
// unhandled-error auto-capture). The user_id is taken from the session, never
// the request body, to prevent forging reports as other users.
func (s *Server) createDebugReport(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		// Defence in depth — the route is mounted under RequireAuth, so this
		// should never fire in practice.
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	// Cap the body at 256 KB (tighter than the 1 MB global cap).
	r.Body = http.MaxBytesReader(w, r.Body, debugReportMaxBodyBytes)

	var req models.CreateDebugReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// http.MaxBytesReader surfaces oversized bodies as *http.MaxBytesError.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body exceeds 256 KB", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// trigger must be "shake" or "auto"
	if req.Trigger != "shake" && req.Trigger != "auto" {
		http.Error(w, "trigger must be \"shake\" or \"auto\"", http.StatusBadRequest)
		return
	}

	// client_id must be a valid UUID
	if req.ClientID == "" {
		http.Error(w, "client_id is required", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(req.ClientID); err != nil {
		http.Error(w, "client_id must be a valid UUID", http.StatusBadRequest)
		return
	}

	// Basic non-empty checks on the other required top-level scalars.
	if req.AppVersion == "" {
		http.Error(w, "app_version is required", http.StatusBadRequest)
		return
	}
	if req.Platform == "" {
		http.Error(w, "platform is required", http.StatusBadRequest)
		return
	}
	if req.Platform != "android" && req.Platform != "ios" && req.Platform != "web" {
		http.Error(w, "platform must be \"android\", \"ios\", or \"web\"", http.StatusBadRequest)
		return
	}

	// Cap the description server-side; frontend limits input at 2000 chars.
	if len(req.Description) > debugReportMaxDescriptionChars {
		http.Error(w, "description must be <= 4096 characters", http.StatusBadRequest)
		return
	}

	report := &models.DebugReport{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		ClientID:    req.ClientID,
		Trigger:     req.Trigger,
		AppVersion:  req.AppVersion,
		Platform:    req.Platform,
		Device:      req.Device,
		State:       req.State,
		Description: req.Description,
		Breadcrumbs: req.Breadcrumbs,
	}

	if err := s.db.CreateDebugReport(report); err != nil {
		serverError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, report)
}
