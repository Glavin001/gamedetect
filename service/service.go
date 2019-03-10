package service /* import "s32x.com/gamedetect/service" */

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"text/template"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"s32x.com/gamedetect/classifier"
)

// Service is a struct that contains everything needed to perform image
// predictions
type Service struct {
	env, domain string
	mu          sync.Mutex
	classifier  *classifier.Classifier
	testResults []Result
}

// NewService creates a new Service reference using the given service params
func NewService(env, domain, graphPath, labelsPath string) (*Service, error) {
	// Create the game classifier using it's default config
	c, err := classifier.NewClassifier(graphPath, labelsPath)
	if err != nil {
		return nil, err
	}
	return &Service{env: env, domain: domain, classifier: c}, nil
}

// Close closes the Service by closing all it's closers ;)
func (s *Service) Close() error { return s.classifier.Close() }

// Start begins serving the generated Service on the passed port
func (s *Service) Start(port string) {
	// Process the testdata
	go s.TestData("service/static/test")

	// Create a new echo Echo and bind all middleware
	e := echo.New()
	e.HideBanner = true
	e.Renderer = &Template{
		templates: template.Must(template.ParseGlob("service/templates/*.html")),
	}

	// Configure SSL, WWW, and Host based redirects if being hosted in a
	// production environment
	if strings.Contains(strings.ToLower(s.env), "prod") {
		e.Pre(middleware.HTTPSNonWWWRedirect())
		e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				if c.Request().Host == s.domain {
					return next(c)
				}
				return c.Redirect(http.StatusPermanentRedirect,
					c.Scheme()+"://"+s.domain)
			}
		})
		e.Pre(middleware.CORS())
	}

	// Bind all middleware
	e.Pre(middleware.RemoveTrailingSlashWithConfig(
		middleware.TrailingSlashConfig{
			RedirectCode: http.StatusPermanentRedirect,
		}))
	e.Pre(middleware.Secure())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.Gzip())

	// Create the static file endpoints
	e.Static("*", "service/static")

	// Bind all API endpoints
	e.POST("/", s.Classify)
	e.GET("/", s.Index)

	// Listen and Serve
	log.Printf("Starting service on port %v\n", port)
	e.Logger.Fatal(e.Start(":" + port))
}
