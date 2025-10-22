package userstate

import "sync"

type User struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	Password     string `json:"-"`
	IsModerator  bool   `json:"is_moderator"`
}

type store struct {
	mu       sync.Mutex
	users    map[string]*User
	nextID   int
	loggedIn *User
}

var s = &store{users: make(map[string]*User), nextID: 1}

func init() {
    s.users["admin"] = &User{ID: s.nextID, Username: "admin", Password: "demo123", IsModerator: true}
    s.nextID++
    s.users["moderator"] = &User{ID: s.nextID, Username: "moderator", Password: "demo123", IsModerator: true}
    s.nextID++
    s.users["user1"] = &User{ID: s.nextID, Username: "user1", Password: "demo123", IsModerator: false}
    s.nextID++
    s.loggedIn = nil
}

func Register(username, password string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return nil, false
	}
	isMod := username == "moderator" || username == "admin"
	u := &User{ID: s.nextID, Username: username, Password: password, IsModerator: isMod}
	s.nextID++
	s.users[username] = u
	return u, true
}

func Login(username, password string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[username]
	if !ok || u.Password != password {
		return nil, false
	}
	s.loggedIn = u
	return u, true
}

func Logout() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loggedIn = nil
}

func Me() (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loggedIn == nil {
		return nil, false
	}
	return &User{ID: s.loggedIn.ID, Username: s.loggedIn.Username, IsModerator: s.loggedIn.IsModerator}, true
}
