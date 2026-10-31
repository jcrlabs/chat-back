package http

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jcrlabs/chat-back/internal/app"
	"github.com/jcrlabs/chat-back/internal/middleware"
)

type userHandler struct {
	svc *app.UserService
}

func newUserHandler(svc *app.UserService) *userHandler {
	return &userHandler{svc: svc}
}

func (h *userHandler) me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	user, err := h.svc.GetByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errBody("user not found"))
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *userHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid body"))
		return
	}
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	if len([]rune(req.DisplayName)) > 32 {
		writeJSON(w, http.StatusBadRequest, errBody("display_name too long (max 32)"))
		return
	}
	if err := h.svc.UpdateProfile(r.Context(), userID, req.DisplayName); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	user, _ := h.svc.GetByID(r.Context(), userID)
	writeJSON(w, http.StatusOK, user)
}

func (h *userHandler) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("file too large (max 2MB)"))
		return
	}
	file, header, err := r.FormFile("avatar")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("missing avatar field"))
		return
	}
	defer file.Close()

	ct := header.Header.Get("Content-Type")
	if ct != "image/jpeg" && ct != "image/png" {
		writeJSON(w, http.StatusBadRequest, errBody("only jpeg and png supported"))
		return
	}

	img, _, err := image.Decode(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("invalid image"))
		return
	}

	resized := resizeTo128(img)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: 85}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}

	if err := h.svc.SaveAvatar(r.Context(), userID, buf.Bytes(), "image/jpeg"); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("internal"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"avatar_url": "/api/users/" + userID.String() + "/avatar",
	})
}

func (h *userHandler) serveAvatar(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, mime, err := h.svc.GetAvatar(r.Context(), id)
	if err != nil || len(data) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func resizeTo128(src image.Image) image.Image {
	const size = 128
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dst.Set(x, y, src.At(sb.Min.X+x*sw/size, sb.Min.Y+y*sh/size))
		}
	}
	return dst
}
