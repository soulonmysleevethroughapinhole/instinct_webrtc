package agent

import "strings"

type UserList []*User

func (u UserList) Len() int      { return len(u) }
func (u UserList) Swap(i, j int) { u[i], u[j] = u[j], u[i] }
func (u UserList) Less(i, j int) bool {
	return strings.ToLower(u[i].N) < strings.ToLower(u[j].N)
}

type User struct {
	ID int
	N  string // Nickname
	C  int    // Channel
}
