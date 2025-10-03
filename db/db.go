package db

import (
	"fmt"

	"github.com/StellaShiina/wireguard-ui/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

type Server struct {
	UUID       string `gorm:"type:uuid;primaryKey" json:"UUID"`
	PublicIP   string `gorm:"type:inet;not null" json:"PublicIP"`
	Port       int    `gorm:"not null" json:"Port"`
	EnableIPv6 bool   `gorm:"not null" json:"EnableIPv6"`
	SubnetV4   string `gorm:"type:cidr;not null" json:"SubnetV4"`
	SubnetV6   string `gorm:"type:cidr;not null" json:"SubnetV6"`
	PrivateKey string `gorm:"not null" json:"-"`
	PublicKey  string `gorm:"not null" json:"-"`
}

type Peer struct {
	UUID       string  `gorm:"type:uuid;primaryKey" json:"UUID"`
	IPv4       *string `gorm:"type:cidr;unique" json:"IPv4"`
	IPv6       *string `gorm:"type:cidr;unique" json:"IPv6"`
	PrivateKey string  `gorm:"not null" json:"-"`
	PublicKey  string  `gorm:"not null" json:"-"`
	Name       *string `json:"Name"`
}

func (Server) TableName() string { return "server" }
func (Peer) TableName() string   { return "peer" }

func Init(cfg *config.Config) error {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}
	DB = db
	return nil
}
