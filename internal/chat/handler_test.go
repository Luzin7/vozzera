package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Luzin7/vozzera-backend/internal/shared/httpx"
	"github.com/google/uuid"
)

type fakeRoomAccess struct {
	allowed bool
}

func (f fakeRoomAccess) CanAccess(context.Context, uuid.UUID, httpx.UserClaims) (bool, error) {
	return f.allowed, nil
}

func newChatMux(repo *fakeRepo, access RoomAccess) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterHandlers(mux, ChatDeps{
		Repo:       repo,
		RoomAccess: access,
		Publisher:  &fakePublisher{},
		AuthMW: httpx.Auth(func(context.Context, string) (httpx.UserClaims, error) {
			return httpx.UserClaims{UserID: uuid.New(), Role: httpx.RoleUser}, nil
		}),
	})
	return mux
}

func authenticatedRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.AddCookie(&http.Cookie{Name: httpx.SessionCookieName, Value: uuid.NewString()})
	return req
}

func TestHandleUpdateMessage_DeniesRoomWithoutAccess(t *testing.T) {
	repo := newFakeRepo()
	edited := false
	repo.updateMessage = func(context.Context, UpdateMessageParams) (UpdateMessageRow, error) {
		edited = true
		return UpdateMessageRow{}, nil
	}

	w := httptest.NewRecorder()
	path := "/api/rooms/" + uuid.NewString() + "/messages/" + uuid.NewString()
	newChatMux(repo, fakeRoomAccess{allowed: false}).ServeHTTP(w, authenticatedRequest(http.MethodPatch, path, `{"content":"editado"}`))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if edited {
		t.Error("mensagem editada numa sala sem acesso")
	}
}

func TestHandleDeleteMessage_DeniesRoomWithoutAccess(t *testing.T) {
	repo := newFakeRepo()
	deleted := false
	repo.deleteMessage = func(context.Context, DeleteMessageParams) (DeleteMessageRow, error) {
		deleted = true
		return DeleteMessageRow{}, nil
	}

	w := httptest.NewRecorder()
	path := "/api/rooms/" + uuid.NewString() + "/messages/" + uuid.NewString()
	newChatMux(repo, fakeRoomAccess{allowed: false}).ServeHTTP(w, authenticatedRequest(http.MethodDelete, path, ""))

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if deleted {
		t.Error("mensagem apagada numa sala sem acesso")
	}
}
