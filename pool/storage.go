package pool

func Save(name string, config Config) error {
	return sqlSetPool(name, config)
}

func Load(name string) (Config, error) {
	return sqlGetPool(name)
}
