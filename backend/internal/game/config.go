package game

import (
	"errors"
)

func (c Config) Validate() error {
	if err := c.Rules.Validate(); err != nil {
		return err
	}
	if err := c.TimeControl.Validate(); err != nil {
		return err
	}
	if c.Black.UserID == nil || c.White.UserID == nil || *c.Black.UserID == *c.White.UserID {
		return errors.New("game requires two distinct players")
	}
	switch c.Mode {
	case "casual", "ranked", "friend", "tournament":
	default:
		return errors.New("unsupported live game mode")
	}
	return nil
}
