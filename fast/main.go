package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aeekayy/systems/fast/db"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// ShortenURL the object that should returned when we return a shorten URL
type ShortenURL struct {
	URI            string `json:"uri" yaml:"uri"`
	ShortenURL     string `json:"shorten_url" yaml:"shorten_url"`
	ShortenLongURL string `json:"shorten_long_url" yaml:"shorten_long_url"`
}

// ShortenURLRequest web request for shorten URL. All we need is the
// url that we want to shorten
type ShortenURLRequest struct {
	URL string `json:"url" yaml:"url"`
}

// URLJSON JSON object for database entries. This should be used to track requests to
// the system
type URLJSON struct {
	Agent   string `json:"agent,omitempty" yaml:"agent,omitempty"`
	Referer string `json:"referer,omitempty" yaml:"referer,omitempty"`
}

const (
	defaultHTTPPort       = 8080                     // The default web port. This should move to the configuration file
	defaultDomainName     = "fast.aeekay.co"         // The default domain name
	defaultLongDomainName = "https://fast.aeekay.co" // The full host with protocol
	letterBytes           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits         = 6                    // 6 bits to represent a letter index
	letterIdxMask         = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax          = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	uriStringCnt          = 8                    // The number of characters in the uri
)

var (
	defaultReservedList = map[string]bool{
		"ping":  true,
		"error": true,
	}
	src = rand.NewSource(time.Now().UnixNano())
)

func main() {
	// retrieve the configuration using viper
	viper.SetConfigName("fast")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fmt.Println("the configuration file was not found")
		} else {
			panic(fmt.Errorf("fatal error config file: %w", err))
		}
	}

	// start the logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	sugar := logger.Sugar()

	// start the db connection
	ctx := context.Background()
	env := getenv("ENV", "dev")
	dbUser := viper.GetString(fmt.Sprintf("%s.db.user", env))
	dbPass := viper.GetString(fmt.Sprintf("%s.db.pass", env))
	dbHost := viper.GetString(fmt.Sprintf("%s.db.host", env))
	dbName := viper.GetString(fmt.Sprintf("%s.db.name", env))
	dbParams := viper.GetString(fmt.Sprintf("%s.db.params", env))
	dbConn, err := db.DBConnect(ctx, dbUser, dbPass, dbHost, dbName, dbParams)

	if err != nil {
		sugar.Fatalf("couldn't connect to the database: %w", err)
	}

	defer dbConn.Close(ctx)

	r := gin.Default()

	r.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	r.GET("/:short_uri", func(c *gin.Context) {
		shortenURI := c.Param("short_uri")

		val, isPresent := defaultReservedList[shortenURI]
		if isPresent && val {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid key for uri",
			})
			return
		}

		var originalURL string
		err = dbConn.QueryRow(ctx, "SELECT original_url FROM urls WHERE uri = $1 LIMIT 1;", shortenURI).Scan(&originalURL)
		if err != nil {
			sugar.Errorf("error retrieving URI: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("error retrieve URI: %s", err),
			})
			return
		}

		c.Redirect(http.StatusMovedPermanently, originalURL)
	})

	r.POST("/api/v1/shorten", func(c *gin.Context) {
		var json ShortenURLRequest
		if err := c.ShouldBindJSON(&json); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unable to retrieve data"})
			return
		}

		generatedURL, err := GenerateURL(json.URL)
		if err != nil {
			sugar.Errorf("error creating URL: %s", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("error creating URL: %s", err),
			})
			return
		}

		details := URLJSON{}
		details.Referer = c.Request.Header.Get("referer")
		details.Agent = "test"

		_, err = dbConn.Query(ctx, "INSERT INTO urls(original_url, uri,raw_json) VALUES($1, $2, $3);", json.URL, generatedURL.URI, details)
		if err != nil {
			sugar.Errorf("error creating URL: %w", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("error creating URL: %s", err),
			})
			return
		}

		sugar.Infof("created new url: %s", generatedURL.ShortenLongURL)
		c.JSON(http.StatusOK, gin.H{
			"data": generatedURL,
		})
	})

	sugar.Info("starting web server")
	r.Run(fmt.Sprintf(":%d", defaultHTTPPort))
}

// GenerateURL generate a shorten URL.
func GenerateURL(originalURL string) (*ShortenURL, error) {
	_, err := url.ParseRequestURI(originalURL)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("couldn't parse url: %s", err))
	}

	uri := RandStringBytesMaskImprSrcSB(uriStringCnt)

	return &ShortenURL{
		ShortenURL:     fmt.Sprintf("%s/%s", defaultDomainName, uri),
		ShortenLongURL: fmt.Sprintf("%s/%s", defaultLongDomainName, uri),
		URI:            uri,
	}, nil
}

// RandStringBytesMaskImprSrcSB generate a random character string. The
// string should be a alpha string with capitalized and lowercase characters
// n is the number of characters in the string
func RandStringBytesMaskImprSrcSB(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

// Scan Make the Attrs struct implement the sql.Scanner interface. This method
// simply decodes a JSON-encoded value into the struct fields.
func (a *URLJSON) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(b, &a)
}

// getenv get the desired environment variable or get the default
// which is the fallback
func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}
