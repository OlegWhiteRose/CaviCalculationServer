package userstate

import "sync"

type User struct {
	Username    string `json:"username"`
	Password    string `json:"-"`
	IsModerator bool   `json:"is_moderator"`
}

type store struct {
	mu       sync.Mutex
	users    map[string]*User
	loggedIn *User
}

var s = &store{users: make(map[string]*User)}

func init() {
	s.users["admin"] = &User{Username: "admin", Password: "demo123", IsModerator: true}
	s.users["moderator"] = &User{Username: "moderator", Password: "demo123", IsModerator: true}
	s.users["user1"] = &User{Username: "user1", Password: "demo123", IsModerator: false}
	s.loggedIn = nil
}

func Register(username, password string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[username]; ok {
		return nil, false
	}
	isMod := username == "moderator" || username == "admin"
	u := &User{Username: username, Password: password, IsModerator: isMod}
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
	return &User{Username: s.loggedIn.Username, IsModerator: s.loggedIn.IsModerator}, true
}

func UpdateMe(newUsername, newPassword *string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loggedIn == nil {
		return nil, false
	}
	oldUsername := s.loggedIn.Username
	if newUsername != nil && *newUsername != "" && *newUsername != oldUsername {
		if _, exists := s.users[*newUsername]; exists {
			return nil, false
		}
		delete(s.users, oldUsername)
		s.loggedIn.Username = *newUsername
		s.users[*newUsername] = s.loggedIn
	}
	if newPassword != nil && *newPassword != "" {
		s.loggedIn.Password = *newPassword
	}
	return &User{Username: s.loggedIn.Username, IsModerator: s.loggedIn.IsModerator}, true
}
