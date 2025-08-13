package main

const (
	maxPeekBytesForSniff = 1 << 20  // 1 MiB for MIME/encoding detection
	maxHashBytes         = 32 << 20 // 32 MiB hashing cap

	defaultReadMaxBytes     = 64 * 1024
	defaultPeekMaxBytes     = 4 * 1024
	defaultListMaxEntries   = 1000
	defaultGlobMaxResults   = 1000
	defaultSearchMaxResults = 100
)
