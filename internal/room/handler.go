package room

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/Luzin7/vozzera-backend/internal/shared/realtime"
	"github.com/google/uuid"
)

type createRoomRequest struct {
	Name     string `json:"name"`
	HasVoice bool   `json:"has_voice"`
}

type updateRoomRequest struct {
	Name      string `json:"name"`
	StaffOnly *bool  `json:"staff_only"`
}

type RoomDeps struct {
	Repo      Repository
	Publisher realtime.Publisher
	Revoker   TopicRevoker
	AuthMW    func(http.Handler) http.Handler
}

type Handler struct {
	listRooms  *ListRoomsService
	createRoom *CreateRoomService
	updateRoom *UpdateRoomService
	deleteRoom *DeleteRoomService
}

func RegisterHandlers(mux *http.ServeMux, deps RoomDeps) {
	h := &Handler{
		listRooms:  NewListRoomsService(deps.Repo),
		createRoom: NewCreateRoomService(deps.Repo, deps.Publisher),
		updateRoom: NewUpdateRoomService(deps.Repo, deps.Publisher, deps.Revoker),
		deleteRoom: NewDeleteRoomService(deps.Repo, deps.Publisher),
	}

	mux.Handle("GET /api/rooms", deps.AuthMW(http.HandlerFunc(h.handleListRooms)))
	mux.Handle("POST /api/rooms", deps.AuthMW(http.HandlerFunc(h.handleCreateRoom)))
	mux.Handle("PATCH /api/rooms/{id}", deps.AuthMW(http.HandlerFunc(h.handleUpdateRoom)))
	mux.Handle("DELETE /api/rooms/{id}", deps.AuthMW(http.HandlerFunc(h.handleDeleteRoom)))
}

func (h *Handler) handleListRooms(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	filter, err := parseRoomFilter(r)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	out, err := h.listRooms.Execute(r.Context(), claims, filter)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, RoomsPresenter(out.Rooms))
}

func (h *Handler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.createRoom.Execute(r.Context(), CreateRoomInput{
		Name:     req.Name,
		HasVoice: req.HasVoice,
		Claims:   claims,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, RoomPresenter(out.Room))
}

func (h *Handler) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req updateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Payload inválido", http.StatusBadRequest)
		return
	}

	out, err := h.updateRoom.Execute(r.Context(), UpdateRoomInput{
		ID:        roomID,
		Name:      req.Name,
		StaffOnly: req.StaffOnly,
		Claims:    claims,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, RoomPresenter(out.Room))
}

func (h *Handler) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	claims, ok := httpx.UserFromContext(r.Context())
	if !ok {
		http.Error(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	roomID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "ID de sala inválido", http.StatusBadRequest)
		return
	}

	if err := h.deleteRoom.Execute(r.Context(), DeleteRoomInput{ID: roomID, Claims: claims}); err != nil {
		httpx.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseRoomFilter(r *http.Request) (RoomFilter, error) {
	raw := r.URL.Query().Get("has_voice")
	if raw == "" {
		return RoomFilter{}, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return RoomFilter{}, ErrInvalidFilter
	}
	return RoomFilter{HasVoice: &value}, nil
}
