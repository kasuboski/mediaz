package download

// SubmissionRejectedError means Add definitely did not create a job. Network,
// decode, and post-add status errors must NOT use this type: their outcome is
// ambiguous and callers must not blindly retry them.
type SubmissionRejectedError struct{ Err error }

func (e *SubmissionRejectedError) Error() string { return e.Err.Error() }
func (e *SubmissionRejectedError) Unwrap() error { return e.Err }
