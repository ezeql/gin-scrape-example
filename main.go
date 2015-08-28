package main

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/gin-gonic/gin"
	"github.com/yhat/scrape"
)

type Movie struct {
	Title       string   `json:"title"`
	Poster      string   `json:"poster"`
	ReleaseYear int      `json:"release_year"`
	Actors      []string `json:"actors"`
	SimilarIDs  []string `json:"similar_ids"`
}

func validateAndFormatAmazonID(ama string) (string, bool) {
	const (
		AmazonIDLength = len("B011J35W5O")
		Alphanumeric   = "^[a-zA-Z0-9]+$" // simple but yet robbed from https://github.com/asaskevich/govalidator
	)

	var rxAlphanumeric = regexp.MustCompile(Alphanumeric)

	parsedID := strings.ToUpper(ama)

	invalid := len(parsedID) != AmazonIDLength || parsedID[0] != 'B' || !rxAlphanumeric.MatchString(parsedID)

	return parsedID, !invalid //return ID, and boolean for valid IDS
}

func main() {

	router := gin.Default()
	router.GET("/movie/amazon/:amazon_id", func(c *gin.Context) {

		id, valid := validateAndFormatAmazonID(c.Param("amazon_id"))

		if !valid {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "invalid amazon id",
				"id":    id,
			})
			return
		}

		resp, err := http.Get("http://www.amazon.de/gp/product/" + id)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err,
			})
			return
		}

		//item does not exist in amazon.de
		if resp.StatusCode == http.StatusNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "product not available",
			})
			return
		}

		root, err := html.Parse(resp.Body)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err,
			})
			return
		}

		actorsMatcher := func(n *html.Node) bool {
			if n.DataAtom == atom.Dd && n.Parent != nil &&
				n.PrevSibling != nil && n.PrevSibling.PrevSibling != nil {
				return scrape.Attr(n.Parent, "class") == "dv-meta-info size-small" &&
					scrape.Text(n.PrevSibling.PrevSibling) == "Darsteller:"
			}
			return false
		}

		posterMatcher := func(n *html.Node) bool {
			if n.DataAtom == atom.Img && n.Parent != nil {
				return scrape.Attr(n.Parent, "class") == "dp-meta-icon-container"
			}
			return false
		}

		//NOTE: Since this is a demo, I assume matchers will always hit a result

		movie := &Movie{}

		titleNode, _ := scrape.Find(root, scrape.ById("aiv-content-title"))
		movie.Title = scrape.Text(titleNode.FirstChild)

		releaseYearNode, _ := scrape.Find(root, scrape.ByClass("release-year"))
		year, _ := strconv.Atoi(scrape.Text(releaseYearNode))
		movie.ReleaseYear = year

		actorsNode, _ := scrape.Find(root, actorsMatcher)
		movie.Actors = strings.Split(scrape.Text(actorsNode), ",")

		posterNode, _ := scrape.Find(root, posterMatcher)
		movie.Poster = scrape.Attr(posterNode, "src")

		movieNodes := scrape.FindAll(root, scrape.ByClass("downloadable_movie"))
		ids := make([]string, len(movieNodes))
		for i, movieNode := range movieNodes {
			ids[i] = scrape.Attr(movieNode, "data-asin")
		}
		movie.SimilarIDs = ids

		c.JSON(http.StatusOK, movie)
	})

	router.Run(":8080")
}
