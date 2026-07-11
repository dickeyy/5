package store

// registeredMigrations returns the immutable ordered production migration registry.
func registeredMigrations() []migration {
	return []migration{
		migration0001InitialV5Schema(),
	}
}
