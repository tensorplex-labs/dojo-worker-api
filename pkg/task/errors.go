package task

type NotFoundError struct {
	ID string
}

func (e *NotFoundError) Error() string {
	return "task with ID " + e.ID + " not found"
}

// IsNotFoundError checks if an error is a TaskNotFoundError
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}
