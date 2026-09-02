package notify

import "errors"

// errorsAs wraps errors.As so the delivery loop reads cleanly.
func errorsAs(err error, target any) bool { return errors.As(err, target) }
