package testutil

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/segmentio/ksuid"

	amv1 "github.com/openshift-online/ocm-sdk-go/accountsmgmt/v1"

	"github.com/openshift-online/rh-trex/pkg/config"
	"github.com/openshift-online/rh-trex/pkg/db"
	"github.com/openshift-online/rh-trex/pkg/testutil/mocks"
	"github.com/openshift-online/rh-trex/pkg/trex"
)

const (
	JwkKID = "uhctestkey"
	JwkAlg = "RS256"
)

type BaseHelper struct {
	Ctx           context.Context
	DBFactory     db.SessionFactory
	AppConfig     *config.ApplicationConfig
	JWTPrivateKey *rsa.PrivateKey
	JWTCA         *rsa.PublicKey
	T             *testing.T
}

func NewBaseHelper(appConfig *config.ApplicationConfig, dbFactory db.SessionFactory) *BaseHelper {
	jwtKey, jwtCA, err := ParseJWTKeys()
	if err != nil {
		fmt.Println("Unable to read JWT keys - this may affect tests that make authenticated server requests")
	}
	return &BaseHelper{
		AppConfig:     appConfig,
		DBFactory:     dbFactory,
		JWTPrivateKey: jwtKey,
		JWTCA:         jwtCA,
	}
}

func (h *BaseHelper) NewID() string {
	return ksuid.New().String()
}

func (h *BaseHelper) NewUUID() string {
	return uuid.New().String()
}

func (h *BaseHelper) RestURL(path string) string {
	protocol := "http"
	if h.AppConfig.Server.EnableHTTPS {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s%s%s", protocol, h.AppConfig.Server.BindAddress, trex.GetConfig().BasePath, path)
}

func (h *BaseHelper) MetricsURL(path string) string {
	return fmt.Sprintf("http://%s%s", h.AppConfig.Metrics.BindAddress, path)
}

func (h *BaseHelper) HealthCheckURL(path string) string {
	return fmt.Sprintf("http://%s%s", h.AppConfig.HealthCheck.BindAddress, path)
}

func (h *BaseHelper) NewRandAccount() *amv1.Account {
	return h.NewAccount(h.NewID(), faker.Name(), faker.Email())
}

func (h *BaseHelper) NewAccount(username, name, email string) *amv1.Account {
	var firstName string
	var lastName string
	names := strings.SplitN(name, " ", 2)
	if len(names) < 2 {
		firstName = name
		lastName = ""
	} else {
		firstName = names[0]
		lastName = names[1]
	}

	builder := amv1.NewAccount().
		Username(username).
		FirstName(firstName).
		LastName(lastName).
		Email(email)

	acct, err := builder.Build()
	if err != nil {
		h.T.Errorf("Unable to build account: %s", err)
	}
	return acct
}

func (h *BaseHelper) StartJWKCertServerMock() (jwkURL string, teardown func() error) {
	jwkURL, teardown = mocks.NewJWKCertServerMock(h.T, h.JWTCA, JwkKID, JwkAlg)
	h.AppConfig.Server.JwkCertURL = jwkURL
	return jwkURL, teardown
}

func (h *BaseHelper) DeleteAll(table interface{}) {
	g2 := h.DBFactory.New(context.Background())
	err := g2.Model(table).Unscoped().Delete(table).Error
	if err != nil {
		h.T.Errorf("error deleting from table %v: %v", table, err)
	}
}

func (h *BaseHelper) Delete(obj interface{}) {
	g2 := h.DBFactory.New(context.Background())
	err := g2.Unscoped().Delete(obj).Error
	if err != nil {
		h.T.Errorf("error deleting object %v: %v", obj, err)
	}
}

func (h *BaseHelper) SkipIfShort() {
	if testing.Short() {
		h.T.Skip("Skipping execution of test in short mode")
	}
}

func (h *BaseHelper) Count(table string) int64 {
	g2 := h.DBFactory.New(context.Background())
	var count int64
	err := g2.Table(table).Count(&count).Error
	if err != nil {
		h.T.Errorf("error getting count for table %s: %v", table, err)
	}
	return count
}

func (h *BaseHelper) MigrateDB() error {
	return db.Migrate(h.DBFactory.New(context.Background()))
}

func (h *BaseHelper) MigrateDBTo(migrationID string) {
	db.MigrateTo(h.DBFactory, migrationID)
}

func (h *BaseHelper) CleanDB() error {
	g2 := h.DBFactory.New(context.Background())

	var tables []string
	err := g2.Raw(`
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
	`).Scan(&tables).Error
	if err != nil {
		h.T.Errorf("error querying tables: %v", err)
		return err
	}

	for _, table := range tables {
		if err := g2.Migrator().DropTable(table); err != nil {
			h.T.Errorf("error dropping table %s: %v", table, err)
			return err
		}
	}
	return nil
}

func (h *BaseHelper) ResetDB() error {
	if err := h.CleanDB(); err != nil {
		return err
	}

	if err := h.MigrateDB(); err != nil {
		return err
	}

	return nil
}

func (h *BaseHelper) CreateJWTString(account *amv1.Account) string {
	claims := jwt.MapClaims{
		"iss":        h.AppConfig.OCM.TokenURL,
		"username":   strings.ToLower(account.Username()),
		"first_name": account.FirstName(),
		"last_name":  account.LastName(),
		"typ":        "Bearer",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(1 * time.Hour).Unix(),
	}
	if account.Email() != "" {
		claims["email"] = account.Email()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = JwkKID

	signedToken, err := token.SignedString(h.JWTPrivateKey)
	if err != nil {
		h.T.Errorf("Unable to sign test jwt: %s", err)
		return ""
	}
	return signedToken
}

func (h *BaseHelper) CreateJWTToken(account *amv1.Account) *jwt.Token {
	tokenString := h.CreateJWTString(account)

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return h.JWTCA, nil
	})
	if err != nil {
		h.T.Errorf("Unable to parse signed jwt: %s", err)
		return nil
	}
	return token
}

func ParseJWTKeys() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateBytes, err := PrivateKeyBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode JWT private key: %s", err)
	}
	pubBytes, err := PublicKeyBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode JWT public key: %s", err)
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEMWithPassword(privateBytes, "passwd")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse JWT private key: %s", err)
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse JWT ca: %s", err)
	}

	return privateKey, pubKey, nil
}

func PrivateKeyBytes() ([]byte, error) {
	s := `LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpQcm9jLVR5cGU6IDQsRU5DUllQVEVECkRF` +
		`Sy1JbmZvOiBERVMtRURFMy1DQkMsMkU2NTExOEU2QzdCNTIwNwoKN2NZVVRXNFpCZG1WWjRJTEIw` +
		`OGhjVGRtNWliMEUwemN5K0k3cEhwTlFmSkh0STdCSjRvbXlzNVMxOXVmSlBCSgpJellqZU83b1RW` +
		`cUkzN0Y2RVVtalpxRzRXVkUyVVFiUURrb3NaYlpOODJPNElwdTFsRkFQRWJ3anFlUE1LdWZ6CnNu` +
		`U1FIS2ZuYnl5RFBFVk5sSmJzMTlOWEM4djZnK3BRYXk1ckgvSTZOMmlCeGdzVG11ZW1aNTRFaE5R` +
		`TVp5RU4KUi9DaWhlQXJXRUg5SDgvNGhkMmdjOVRiMnMwTXdHSElMTDRrYmJObTV0cDN4dzRpazdP` +
		`WVdOcmozbStuRzZYYgp2S1hoMnhFYW5BWkF5TVhUcURKVEhkbjcvQ0VxdXNRUEpqWkdWK01mMWtq` +
		`S3U3cDRxY1hGbklYUDVJTG5UVzdiCmxIb1dDNGV3ZUR6S09NUnpYbWJBQkVWU1V2eDJTbVBsNFRj` +
		`b0M1TDFTQ0FIRW1aYUtiYVk3UzVsNTN1NmdsMGYKVUx1UWJ0N0hyM1RIem5sTkZLa0dUMS95Vk50` +
		`MlFPbTFlbVpkNTVMYU5lOEU3WHNOU2xobDBncllRK1VlOEpiYQp4ODVPYXBsdFZqeE05d1ZDd2Jn` +
		`RnlpMDRpaGRLSG85ZSt1WUtlVEdLdjBoVTVPN0hFSDFldjZ0L3MydS9VRzZoClRxRXNZclZwMENN` +
		`SHB0NXVBRjZuWnlLNkdaL0NIVHhoL3J6MWhBRE1vZmVtNTkrZTZ0VnRqblBHQTNFam5KVDgKQk1P` +
		`dy9EMlFJRHhqeGoyR1V6eitZSnA1MEVOaFdyTDlvU0RrRzJuenY0TlZMNzdRSXkrVC8yL2Y0UGdv` +
		`a1VETwpRSmpJZnhQV0U0MGNIR0hwblF0WnZFUG94UDBIM1QwWWhtRVZ3dUp4WDN1YVdPWS84RmEx` +
		`YzdMbjBTd1dkZlY1CmdZdkpWOG82YzNzdW1jcTFPM2FnUERsSEM1TzRJeEc3QVpROENIUkR5QVNv` +
		`Z3pma1k2UDU3OVpPR1lhTzRhbDcKV0ExWUlwc0hzMy8xZjRTQnlNdVdlME5Wa0Zmdlhja2pwcUdy` +
		`QlFwVG1xUXprNmJhYTBWUTBjd1UzWGxrd0hhYwpXQi9mUTRqeWx3RnpaRGNwNUpBbzUzbjZhVTcy` +
		`emdOdkRsR1ROS3dkWFhaSTVVM0pQb2NIMEFpWmdGRldZSkxkCjYzUEpMRG5qeUUzaTZYTVZseGlm` +
		`WEtrWFZ2MFJZU3orQnlTN096OWFDZ25RaE5VOHljditVeHRma1BRaWg1ekUKLzBZMkVFRmtuYWpt` +
		`RkpwTlhjenpGOE9FemFzd21SMEFPamNDaWtsWktSZjYxcmY1ZmFKeEpoaHFLRUVCSnVMNgpvb2RE` +
		`VlJrM09HVTF5UVNCYXpUOG5LM1YrZTZGTW8zdFdrcmEyQlhGQ0QrcEt4VHkwMTRDcDU5UzF3NkYx` +
		`Rmp0CldYN2VNV1NMV2ZRNTZqMmtMTUJIcTVnYjJhcnFscUgzZnNZT1REM1ROakNZRjNTZ3gzMDlr` +
		`VlB1T0s1dnc2MVAKcG5ML0xOM2lHWTQyV1IrOWxmQXlOTjJxajl6dndLd3NjeVlzNStEUFFvUG1j` +
		`UGNWR2Mzdi91NjZiTGNPR2JFVQpPbEdhLzZnZEQ0R0NwNUU0ZlAvN0dibkVZL1BXMmFicXVGaEdC` +
		`K3BWZGwzLzQrMVUvOGtJdGxmV05ab0c0RmhFCmdqTWQ3Z2xtcmRGaU5KRkZwZjVrczFsVlhHcUo0` +
		`bVp4cXRFWnJ4VUV3Y2laam00VjI3YStFMkt5VjlObmtzWjYKeEY0dEdQS0lQc3ZOVFY1bzhacWpp` +
		`YWN4Z2JZbXIyeXdxRFhLQ2dwVS9SV1NoMXNMYXBxU1FxYkgvdzBNcXVVagpWaFZYMFJNWUgvZm9L` +
		`dGphZ1pmL0tPMS9tbkNJVGw4NnRyZUlkYWNoR2dSNHdyL3FxTWpycFBVYVBMQ1JZM0pRCjAwWFVQ` +
		`MU11NllQRTBTbk1ZQVZ4WmhlcUtIbHkzYTFwZzRYcDdZV2xNNjcxb1VPUnMzK1ZFTmZuYkl4Z3Ir` +
		`MkQKVGlKVDlQeHdwZks1M09oN1JCU1dISlpSdUFkTFVYRThERytibDBOL1FrSk02cEZVeFRJMUFR` +
		`PT0KLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0K`

	return base64.StdEncoding.DecodeString(s)
}

func PublicKeyBytes() ([]byte, error) {
	s := `LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUMvekNDQWVlZ0F3SUJBZ0lCQVRBTkJna3Fo` +
		`a2lHOXcwQkFRVUZBREFhTVFzd0NRWURWUVFHRXdKVlV6RUwKTUFrR0ExVUVDZ3dDV2pRd0hoY05N` +
		`VE13T0RJNE1UZ3lPRE0wV2hjTk1qTXdPREk0TVRneU9ETTBXakFhTVFzdwpDUVlEVlFRR0V3SlZV` +
		`ekVMTUFrR0ExVUVDZ3dDV2pRd2dnRWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUJEd0F3CmdnRUtB` +
		`b0lCQVFEZmRPcW90SGQ1NVNZTzBkTHoyb1hlbmd3L3RaK3EzWm1PUGVWbU11T01JWU8vQ3Yxd2sy` +
		`VTAKT0s0cHVnNE9CU0pQaGwwOVpzNkl3QjhOd1BPVTdFRFRnTU9jUVVZQi82UU5DSTFKN1ptMm9M` +
		`dHVjaHp6NHBJYgorbzRaQWhWcHJMaFJ5dnFpOE9US1E3a2ZHZnM1VHV3bW4xTS8wZlFrZnpNeEFE` +
		`cGpPS05nZjB1eTZsTjZ1dGpkClRyUEtLRlVRTmRjNi9UeThFZVRuUUV3VWxzVDJMQVhDZkVLeFRu` +
		`NVJsUmxqRHp0UzdTZmdzOFZMMEZQeTFRaTgKQitkRmNnUllLRnJjcHNWYVoxbEJtWEtzWERSdTVR` +
		`Ui9SZzNmOURScTRHUjFzTkg4UkxZOXVBcE1sMlNOeitzUgo0elJQRzg1Ui9zZTVRMDZHdTBCVVEz` +
		`VVBtNjdFVFZaTEFnTUJBQUdqVURCT01CMEdBMVVkRGdRV0JCUUhaUFRFCnlRVnUvMEkvM1FXaGxU` +
		`eVc3V29UelRBZkJnTlZIU01FR0RBV2dCUUhaUFRFeVFWdS8wSS8zUVdobFR5VzdXb1QKelRBTUJn` +
		`TlZIUk1FQlRBREFRSC9NQTBHQ1NxR1NJYjNEUUVCQlFVQUE0SUJBUURIeHFKOXk0YWxUSDdhZ1ZN` +
		`VwpaZmljL1JicmR2SHd5cStJT3JnRFRvcXlvMHcrSVo2QkNuOXZqdjVpdWhxdTRGb3JPV0RBRnBR` +
		`S1pXMERMQkpFClF5LzcvMCs5cGsyRFBoSzFYemRPb3ZsU3JrUnQrR2NFcEduVVhuekFDWERCYk8w` +
		`K1dyaytoY2pFa1FSUksxYlcKMnJrbkFSSUVKRzlHUytwU2hQOUJxLzBCbU5zTWVwZE5jQmEwejNh` +
		`NUIwZnpGeUNRb1VsWDZSVHF4UncxaDFRdAo1RjAwcGZzcDdTalhWSXZZY2V3SGFOQVNidG8xbjVo` +
		`clN6MVZZOWhMYmExMWl2TDFONFdvV2JtekFMNkJXYWJzCkMyRC9NZW5TVDIvWDZoVEt5R1hwZzNF` +
		`ZzJoM2lMdlV0d2NObnkwaFJLc3RjNzNKbDl4UjNxWGZYS0pIMFRoVGwKcTBncQotLS0tLUVORCBD` +
		`RVJUSUZJQ0FURS0tLS0tCg==`
	return base64.StdEncoding.DecodeString(s)
}
