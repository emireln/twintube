package room

import (
	"encoding/json"
	"strings"
	"time"

	"twintube/internal/db"
	"twintube/internal/utils"
)

type Moment struct {
	ID               string    `json:"id"`
	RoomID           string    `json:"roomId"`
	CreatorClientID  string    `json:"creatorClientId,omitempty"`
	CreatorUserID    string    `json:"creatorUserId,omitempty"`
	CreatorNickname  string    `json:"creatorNickname"`
	VideoID          string    `json:"videoId"`
	Platform         string    `json:"platform,omitempty"`
	AtSeconds        float64   `json:"atSeconds"`
	Label            string    `json:"label,omitempty"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (r *Room) SubmitMoment(from *Client, atSeconds float64, label string) (Moment, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if from == nil {
		return Moment{}, false
	}
	if r.State.MediaKind == "live" || !r.State.Seekable {
		return Moment{}, false
	}
	if atSeconds < 0 {
		atSeconds = 0
	}
	label = strings.TrimSpace(label)
	if len(label) > 120 {
		label = label[:120]
	}
	m := Moment{
		ID:              utils.GenerateRandomID(8),
		RoomID:          r.ID,
		CreatorClientID: from.ID,
		CreatorUserID:   from.UserID,
		CreatorNickname: from.Nickname,
		VideoID:         r.State.VideoID,
		Platform:        r.State.Platform,
		AtSeconds:       atSeconds,
		Label:           label,
		Status:          "pending",
		CreatedAt:       time.Now().UTC(),
	}
	r.Moments = append(r.Moments, m)
	return m, true
}

func (r *Room) setMomentStatus(from *Client, momentID, status string) (Moment, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if from == nil || !from.CanControlPlayback() {
		return Moment{}, false
	}
	for i := range r.Moments {
		if r.Moments[i].ID != momentID {
			continue
		}
		if r.Moments[i].Status != "pending" {
			return Moment{}, false
		}
		r.Moments[i].Status = status
		return r.Moments[i], true
	}
	return Moment{}, false
}

func (r *Room) ApproveMoment(from *Client, momentID string) (Moment, bool) {
	return r.setMomentStatus(from, momentID, "approved")
}

func (r *Room) RejectMoment(from *Client, momentID string) (Moment, bool) {
	return r.setMomentStatus(from, momentID, "rejected")
}

func (r *Room) FindMoment(momentID string) (Moment, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, m := range r.Moments {
		if m.ID == momentID {
			return m, true
		}
	}
	return Moment{}, false
}

func (r *Room) ApprovedMoments() []Moment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Moment, 0)
	for _, m := range r.Moments {
		if m.Status == "approved" {
			out = append(out, m)
		}
	}
	return out
}

func (r *Room) PendingMoments() []Moment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Moment, 0)
	for _, m := range r.Moments {
		if m.Status == "pending" {
			out = append(out, m)
		}
	}
	return out
}

func (r *Room) BroadcastApprovedMoment(m Moment) {
	raw, _ := json.Marshal(m)
	r.deliver(WSMessage{Action: "MOMENT_APPROVED", Payload: raw})
}

func (r *Room) NotifyApproversPending(m Moment) {
	raw, _ := json.Marshal(m)
	msg := WSMessage{Action: "MOMENT_PENDING", Payload: raw}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.Clients {
		if c.CanControlPlayback() {
			select {
			case c.Send <- msg:
			default:
			}
		}
	}
}

func (r *Room) PersistMoment(m Moment) {
	if db.Database == nil || !r.IsPersistent() {
		return
	}
	_ = db.Database.SaveMoment(m.ID, m.RoomID, m.CreatorUserID, m.CreatorNickname, m.VideoID, m.Platform, m.Label, m.Status, m.AtSeconds, m.CreatedAt)
}

func (r *Room) PersistMomentStatus(m Moment) {
	if db.Database == nil || !r.IsPersistent() {
		return
	}
	_ = db.Database.UpdateMomentStatus(m.ID, m.Status)
}
