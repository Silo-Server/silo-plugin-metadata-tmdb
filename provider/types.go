package provider

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

type configResponse struct {
	Images struct {
		SecureBaseURL string   `json:"secure_base_url"`
		PosterSizes   []string `json:"poster_sizes"`
		BackdropSizes []string `json:"backdrop_sizes"`
		ProfileSizes  []string `json:"profile_sizes"`
		StillSizes    []string `json:"still_sizes"`
		LogoSizes     []string `json:"logo_sizes"`
	} `json:"images"`
}

// ---------------------------------------------------------------------------
// Generic paginated response
// ---------------------------------------------------------------------------

type paginatedResponse[T any] struct {
	Page         int `json:"page"`
	TotalPages   int `json:"total_pages"`
	TotalResults int `json:"total_results"`
	Results      []T `json:"results"`
}

// ---------------------------------------------------------------------------
// Search results
// ---------------------------------------------------------------------------

// MovieResult is a single movie from a TMDB search.
type MovieResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	Overview      string  `json:"overview"`
	Popularity    float64 `json:"popularity"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Adult         bool    `json:"adult"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	GenreIDs      []int   `json:"genre_ids"`
}

// TVResult is a single TV show from a TMDB search.
type TVResult struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	OriginalName  string   `json:"original_name"`
	FirstAirDate  string   `json:"first_air_date"`
	Overview      string   `json:"overview"`
	Popularity    float64  `json:"popularity"`
	VoteAverage   float64  `json:"vote_average"`
	VoteCount     int      `json:"vote_count"`
	PosterPath    string   `json:"poster_path"`
	BackdropPath  string   `json:"backdrop_path"`
	GenreIDs      []int    `json:"genre_ids"`
	OriginCountry []string `json:"origin_country"`
}

// TrendingResult is a single item from the TMDB trending endpoint.
// It may be a movie or TV show, distinguished by the MediaType field.
type TrendingResult struct {
	ID        int    `json:"id"`
	MediaType string `json:"media_type"` // "movie" or "tv"
	Title     string `json:"title"`      // populated for movies
	Name      string `json:"name"`       // populated for TV shows
}

// CollectionResult is a normalized item returned from a TMDB collection preset.
type CollectionResult struct {
	ID        int
	MediaType string
	Title     string
}

// ---------------------------------------------------------------------------
// Movie detail (with append_to_response)
// ---------------------------------------------------------------------------

// MovieDetail is the full movie record from /movie/{id}.
type MovieDetail struct {
	ID                  int                  `json:"id"`
	Title               string               `json:"title"`
	OriginalTitle       string               `json:"original_title"`
	Overview            string               `json:"overview"`
	Tagline             string               `json:"tagline"`
	ReleaseDate         string               `json:"release_date"`
	Runtime             int                  `json:"runtime"`
	Status              string               `json:"status"`
	Popularity          float64              `json:"popularity"`
	VoteAverage         float64              `json:"vote_average"`
	VoteCount           int                  `json:"vote_count"`
	Adult               bool                 `json:"adult"`
	IMDbID              string               `json:"imdb_id"`
	OriginalLanguage    string               `json:"original_language"`
	PosterPath          string               `json:"poster_path"`
	BackdropPath        string               `json:"backdrop_path"`
	Genres              []Genre              `json:"genres"`
	ProductionCompanies []Company            `json:"production_companies"`
	ProductionCountries []Country            `json:"production_countries"`
	SpokenLanguages     []Language           `json:"spoken_languages"`
	OriginCountry       []string             `json:"origin_country"`
	BelongsToCollection *Collection          `json:"belongs_to_collection"`
	Credits             *Credits             `json:"credits"`
	ExternalIDs         *ExternalIDs         `json:"external_ids"`
	Images              *ImageSet            `json:"images"`
	Keywords            *MovieKeywords       `json:"keywords"`
	ReleaseDates        *ReleaseDatesWrapper `json:"release_dates"`
}

// ---------------------------------------------------------------------------
// TV detail (with append_to_response)
// ---------------------------------------------------------------------------

// TVDetail is the full TV show record from /tv/{id}.
type TVDetail struct {
	ID                  int                    `json:"id"`
	Name                string                 `json:"name"`
	OriginalName        string                 `json:"original_name"`
	Overview            string                 `json:"overview"`
	Tagline             string                 `json:"tagline"`
	FirstAirDate        string                 `json:"first_air_date"`
	LastAirDate         string                 `json:"last_air_date"`
	Status              string                 `json:"status"`
	Type                string                 `json:"type"`
	Popularity          float64                `json:"popularity"`
	VoteAverage         float64                `json:"vote_average"`
	VoteCount           int                    `json:"vote_count"`
	OriginalLanguage    string                 `json:"original_language"`
	PosterPath          string                 `json:"poster_path"`
	BackdropPath        string                 `json:"backdrop_path"`
	Genres              []Genre                `json:"genres"`
	Networks            []Network              `json:"networks"`
	ProductionCompanies []Company              `json:"production_companies"`
	OriginCountry       []string               `json:"origin_country"`
	SpokenLanguages     []Language             `json:"spoken_languages"`
	Languages           []string               `json:"languages"`
	NumberOfSeasons     int                    `json:"number_of_seasons"`
	NumberOfEpisodes    int                    `json:"number_of_episodes"`
	Seasons             []SeasonSummary        `json:"seasons"`
	CreatedBy           []Creator              `json:"created_by"`
	Credits             *Credits               `json:"credits"`
	ExternalIDs         *ExternalIDs           `json:"external_ids"`
	Images              *ImageSet              `json:"images"`
	Keywords            *TVKeywords            `json:"keywords"`
	ContentRatings      *ContentRatingsWrapper `json:"content_ratings"`
}

// ---------------------------------------------------------------------------
// Season detail (from /tv/{id}/season/{num})
// ---------------------------------------------------------------------------

// SeasonDetail is the full season record with episodes.
type SeasonDetail struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	Overview     string          `json:"overview"`
	AirDate      string          `json:"air_date"`
	SeasonNumber int             `json:"season_number"`
	PosterPath   string          `json:"poster_path"`
	VoteAverage  float64         `json:"vote_average"`
	Episodes     []EpisodeDetail `json:"episodes"`
	ExternalIDs  *ExternalIDs    `json:"external_ids"`
}

// EpisodeDetail is a single episode within a season.
type EpisodeDetail struct {
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Overview      string       `json:"overview"`
	AirDate       string       `json:"air_date"`
	EpisodeNumber int          `json:"episode_number"`
	SeasonNumber  int          `json:"season_number"`
	Runtime       int          `json:"runtime"`
	VoteAverage   float64      `json:"vote_average"`
	VoteCount     int          `json:"vote_count"`
	StillPath     string       `json:"still_path"`
	ShowID        int          `json:"show_id"`
	Crew          []CrewMember `json:"crew"`
	GuestStars    []CastMember `json:"guest_stars"`
}

// ---------------------------------------------------------------------------
// Find by external ID
// ---------------------------------------------------------------------------

// FindResponse is the result of /find/{external_id}.
type FindResponse struct {
	MovieResults     []MovieResult     `json:"movie_results"`
	PersonResults    []PersonResult    `json:"person_results"`
	TVResults        []TVResult        `json:"tv_results"`
	TVEpisodeResults []TVEpisodeResult `json:"tv_episode_results"`
	TVSeasonResults  []TVSeasonResult  `json:"tv_season_results"`
}

// PersonResult is a person returned by the TMDB find endpoint.
type PersonResult struct {
	ID int `json:"id"`
}

// TVEpisodeResult is an episode in a find response.
type TVEpisodeResult struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	EpisodeNumber int    `json:"episode_number"`
	SeasonNumber  int    `json:"season_number"`
	ShowID        int    `json:"show_id"`
}

// TVSeasonResult is a season in a find response.
type TVSeasonResult struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	SeasonNumber int    `json:"season_number"`
	ShowID       int    `json:"show_id"`
}

// ---------------------------------------------------------------------------
// Shared types
// ---------------------------------------------------------------------------

// Genre is a TMDB genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Keyword is a TMDB keyword tag.
type Keyword struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// MovieKeywords wraps movie keyword tags returned by append_to_response=keywords.
type MovieKeywords struct {
	Keywords []Keyword `json:"keywords"`
}

// TVKeywords wraps TV keyword tags returned by append_to_response=keywords.
type TVKeywords struct {
	Results []Keyword `json:"results"`
}

// Credits holds the cast and crew for a movie or TV show.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// CastMember is a single actor credit from TMDB.
type CastMember struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	OriginalName       string `json:"original_name"`
	Character          string `json:"character"`
	ProfilePath        string `json:"profile_path"`
	Order              int    `json:"order"`
	KnownForDepartment string `json:"known_for_department"`
	Gender             int    `json:"gender"`
}

// CrewMember is a single crew credit from TMDB.
type CrewMember struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	OriginalName       string `json:"original_name"`
	Job                string `json:"job"`
	Department         string `json:"department"`
	ProfilePath        string `json:"profile_path"`
	KnownForDepartment string `json:"known_for_department"`
	Gender             int    `json:"gender"`
}

// ExternalIDs holds cross-references to other providers.
type ExternalIDs struct {
	IMDbID     string `json:"imdb_id"`
	TVDBID     int    `json:"tvdb_id"`
	WikidataID string `json:"wikidata_id"`
	FacebookID string `json:"facebook_id"`
	TwitterID  string `json:"twitter_id"`
}

// PersonDetail is the full person record from /person/{id}.
type PersonDetail struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Biography    string       `json:"biography"`
	Birthday     string       `json:"birthday"`
	Deathday     string       `json:"deathday"`
	PlaceOfBirth string       `json:"place_of_birth"`
	Homepage     string       `json:"homepage"`
	ProfilePath  string       `json:"profile_path"`
	ExternalIDs  *ExternalIDs `json:"external_ids"`
}

// Image is a single image from TMDB.
type Image struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	ISO639_1    string  `json:"iso_639_1"`
}

// ImageSet holds categorised images.
type ImageSet struct {
	Backdrops []Image `json:"backdrops"`
	Logos     []Image `json:"logos"`
	Posters   []Image `json:"posters"`
	Stills    []Image `json:"stills"`
}

// Collection is a franchise/collection reference.
type Collection struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
}

// Company is a production company.
type Company struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// Network is a TV network.
type Network struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// Country is a production country.
type Country struct {
	ISO3166_1 string `json:"iso_3166_1"`
	Name      string `json:"name"`
}

// Language is a spoken language.
type Language struct {
	EnglishName string `json:"english_name"`
	ISO639_1    string `json:"iso_639_1"`
	Name        string `json:"name"`
}

// Creator is a TV show creator.
type Creator struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
}

// SeasonSummary is a brief season record within a TV detail response.
type SeasonSummary struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Overview     string  `json:"overview"`
	AirDate      string  `json:"air_date"`
	SeasonNumber int     `json:"season_number"`
	EpisodeCount int     `json:"episode_count"`
	PosterPath   string  `json:"poster_path"`
	VoteAverage  float64 `json:"vote_average"`
}

// ---------------------------------------------------------------------------
// Content ratings / release dates (for extracting certifications)
// ---------------------------------------------------------------------------

// ReleaseDatesWrapper wraps release date results per country.
type ReleaseDatesWrapper struct {
	Results []ReleaseDateCountry `json:"results"`
}

// ReleaseDateCountry groups release dates for a single country.
type ReleaseDateCountry struct {
	ISO3166_1    string        `json:"iso_3166_1"`
	ReleaseDates []ReleaseDate `json:"release_dates"`
}

// ReleaseDate is a single release date entry.
type ReleaseDate struct {
	Certification string `json:"certification"`
	Type          int    `json:"type"` // 1=Premiere, 2=Limited, 3=Theatrical, 4=Digital, 5=Physical, 6=TV
	ReleaseDate   string `json:"release_date"`
}

// ContentRatingsWrapper wraps content rating results per country.
type ContentRatingsWrapper struct {
	Results []ContentRating `json:"results"`
}

// ContentRating is a content/age rating for a specific country.
type ContentRating struct {
	ISO3166_1 string `json:"iso_3166_1"`
	Rating    string `json:"rating"`
}

// ---------------------------------------------------------------------------
// API error response
// ---------------------------------------------------------------------------

type apiError struct {
	StatusMessage string `json:"status_message"`
	StatusCode    int    `json:"status_code"`
}
