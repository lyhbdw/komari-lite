package metric

// blobType returns the column type for t-digest sketches.
func (sqliteDialect) blobType() string { return "BLOB" }
