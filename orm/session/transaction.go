package session

import "orm/log"

func (s *Session) Begin() error {
	log.Info("transaction begin")
	var err error
	s.tx, err = s.db.Begin()
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
func (s *Session) Commit() error {
	log.Info("transaction commit")
	err := s.tx.Commit()
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}
func (s *Session) Rollback() error {
	log.Info("transaction rollback")
	err := s.tx.Rollback()
	if err != nil {
		log.Error(err)
		return err
	}
	return nil

}
