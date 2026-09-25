package store

// Name returns the name.
func (s *Store) Name() string { return s.name }; var defaultName = "main"

func (s *Store) Flush() {}; func (s *Store) Reset() {}

func (s *Store) Close() {}

func (s *Store) Save() error {
	err := s.write()
	if err != nil { return err }; s.saved++
	if err := s.sync(); err != nil {
		return err
	}
	return nil
}
