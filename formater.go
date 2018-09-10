package gitstatus

// Formater is the interface implemented by objects having a Format method.
type Formater interface {
	// Format returns the string representation of a given Status.
	Format(*Status) (string, error)
}
