package services

import (
	"errors"

	"github.com/quackdiscord/bot/lib"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type dbService struct {
	*gorm.DB
}

var DB = &dbService{}

func (s *dbService) Connect() {
	db, err := gorm.Open(mysql.Open(lib.Config.Storage.DBDSN), &gorm.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get sql.DB from gorm")
	}

	if err := sqlDB.Ping(); err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}

	s.DB = db
	log.Info().Msg("Connected to database")
}

func (s *dbService) Ping() error {
	if s.DB == nil {
		return errors.New("database not connected")
	}
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
