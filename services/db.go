package services

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	"github.com/quackdiscord/bot/lib"
	"github.com/rs/zerolog/log"
)

type dbService struct {
	*sql.DB
}

var DB = &dbService{}

func (s *dbService) Connect() {
	db, err := sql.Open("mysql", lib.Config.Storage.DBDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	err = db.Ping()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}

	s.DB = db
	log.Info().Msg("Connected to database")
}
