package room

import (
	"testing"
	"time"

	"twintube/internal/db"
)

func TestVoteSkipMajority(t *testing.T) {
	r := Manager.newRoomShell("vote1", "Vote")
	c1 := &Client{ID: "a", Nickname: "A", JoinedAt: time.Now()}
	c2 := &Client{ID: "b", Nickname: "B", JoinedAt: time.Now()}
	c3 := &Client{ID: "c", Nickname: "C", JoinedAt: time.Now()}
	r.Clients[c1.ID] = c1
	r.Clients[c2.ID] = c2
	r.Clients[c3.ID] = c3
	r.State.VideoID = "vid1"

	votes, needed, passed, ok := r.VoteSkip(c1)
	if !ok || passed || votes != 1 || needed != 2 {
		t.Fatalf("first vote: votes=%d needed=%d passed=%v", votes, needed, passed)
	}
	votes, needed, passed, ok = r.VoteSkip(c2)
	if !ok || !passed || votes != 2 || needed != 2 {
		t.Fatalf("majority vote: votes=%d needed=%d passed=%v", votes, needed, passed)
	}
}

func TestReorderPlaylist(t *testing.T) {
	r := Manager.newRoomShell("ord1", "Order")
	r.Playlist = []db.PlaylistItem{
		{ID: "1", Position: 0},
		{ID: "2", Position: 1},
		{ID: "3", Position: 2},
	}
	out, ok := r.ReorderPlaylist([]string{"3", "1", "2"})
	if !ok {
		t.Fatal("reorder failed")
	}
	if out[0].ID != "3" || out[1].ID != "1" || out[2].ID != "2" {
		t.Fatalf("unexpected order: %#v", out)
	}
	if out[0].Position != 0 || out[2].Position != 2 {
		t.Fatalf("positions not renumbered: %#v", out)
	}
	if _, ok := r.ReorderPlaylist([]string{"3", "1"}); ok {
		t.Fatal("stale order should fail")
	}
}

func TestPromotePrefersCohost(t *testing.T) {
	r := Manager.newRoomShell("host1", "Host")
	viewer := &Client{ID: "v", Nickname: "Viewer", JoinedAt: time.Now().Add(-time.Hour)}
	cohost := &Client{ID: "c", Nickname: "Cohost", IsCohost: true, JoinedAt: time.Now()}
	r.Clients[viewer.ID] = viewer
	r.Clients[cohost.ID] = cohost
	next := r.promoteNewHostLocked()
	if next == nil || next.ID != "c" {
		t.Fatalf("expected cohost promotion, got %#v", next)
	}
}

func TestAssignHostClearsOthers(t *testing.T) {
	r := Manager.newRoomShell("host2", "Host")
	a := &Client{ID: "a", Nickname: "A", IsHost: true, JoinedAt: time.Now().Add(-time.Minute)}
	b := &Client{ID: "b", Nickname: "B", IsCohost: true, JoinedAt: time.Now()}
	r.Clients[a.ID] = a
	r.Clients[b.ID] = b
	r.HostID = a.ID
	r.assignHostLocked(b)
	if r.HostID != b.ID || !b.IsHost || a.IsHost || b.IsCohost {
		t.Fatalf("assignHost failed: host=%s a.host=%v b.host=%v b.cohost=%v", r.HostID, a.IsHost, b.IsHost, b.IsCohost)
	}
}

func TestTransferHostRequiresCurrentHost(t *testing.T) {
	r := Manager.newRoomShell("host3", "Host")
	host := &Client{ID: "h", Nickname: "Host", IsHost: true, JoinedAt: time.Now().Add(-time.Minute)}
	guest := &Client{ID: "g", Nickname: "Guest", JoinedAt: time.Now()}
	r.Clients[host.ID] = host
	r.Clients[guest.ID] = guest
	r.HostID = host.ID

	if _, ok := r.TransferHost(guest, host.ID); ok {
		t.Fatal("guest must not transfer host")
	}
	if nick, ok := r.TransferHost(host, guest.ID); !ok || nick != "Guest" {
		t.Fatalf("host transfer failed: nick=%s ok=%v", nick, ok)
	}
	if r.HostID != guest.ID || !guest.IsHost || host.IsHost {
		t.Fatalf("after transfer: host=%s guest.host=%v host.host=%v", r.HostID, guest.IsHost, host.IsHost)
	}
}

func TestOwnerReclaimWinsOverGuestHost(t *testing.T) {
	r := Manager.newRoomShell("host4", "Owned")
	r.OwnerID = "owner-user"
	guest := &Client{ID: "g", Nickname: "Guest", IsHost: true, JoinedAt: time.Now().Add(-time.Minute)}
	owner := &Client{ID: "o", Nickname: "Owner", UserID: "owner-user", JoinedAt: time.Now()}
	r.Clients[guest.ID] = guest
	r.HostID = guest.ID
	r.Clients[owner.ID] = owner
	r.assignHostLocked(owner)
	if r.HostID != owner.ID || !owner.IsHost || guest.IsHost {
		t.Fatalf("owner reclaim failed: host=%s owner.host=%v guest.host=%v", r.HostID, owner.IsHost, guest.IsHost)
	}
}
