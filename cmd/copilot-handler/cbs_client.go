// CBS API Client - extracted from mcp-cbs-cijfers-open-data
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	cbsBaseURL = "https://datasets.cbs.nl/odata/v1"
)

// CBSClient represents a CBS API client.
type CBSClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewCBSClient creates a new CBS API client.
func NewCBSClient() *CBSClient {
	return &CBSClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    cbsBaseURL,
	}
}

// Catalog represents a CBS data catalog.
type Catalog struct {
	Identifier   string `json:"identifier"`
	Index        int    `json:"index"`
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	Publisher    string `json:"publisher,omitempty"`
	Language     string `json:"language,omitempty"`
	License      string `json:"license,omitempty"`
	Homepage     string `json:"homepage,omitempty"`
	Authority    string `json:"authority,omitempty"`
	ContactPoint string `json:"contactPoint,omitempty"`
}

// CatalogResponse represents the response from the Catalogs endpoint.
type CatalogResponse struct {
	Value []Catalog `json:"value"`
}

// Dataset represents a CBS dataset.
type Dataset struct {
	ID                   string    `json:"id,omitempty"`
	Identifier           string    `json:"identifier"`
	Title                string    `json:"title"`
	Description          string    `json:"-"`
	Modified             time.Time `json:"modified"`
	ReleaseDate          time.Time `json:"releaseDate"`
	ModificationDate     time.Time `json:"modificationDate"`
	Language             string    `json:"language"`
	Catalog              string    `json:"catalog"`
	Version              string    `json:"version"`
	Status               string    `json:"status"`
	ObservationsModified time.Time `json:"observationsModified"`
	ObservationCount     int64     `json:"observationCount"`
	DatasetType          string    `json:"datasetType"`
}

// DatasetResponse represents the response from the Datasets endpoint.
type DatasetResponse struct {
	Value      []Dataset `json:"value"`
	ODataCount *int      `json:"@odata.count,omitempty"`
}

// Dimension represents a dataset dimension.
type Dimension struct {
	Identifier     string `json:"identifier"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Kind           string `json:"kind"`
	ContainsGroups bool   `json:"containsGroups"`
	ContainsCodes  bool   `json:"containsCodes"`
}

// DimensionResponse represents the response from the Dimensions endpoint.
type DimensionResponse struct {
	Value []Dimension `json:"value"`
}

// GetCatalogs retrieves all available CBS catalogs.
func (c *CBSClient) GetCatalogs() ([]Catalog, error) {
	url := fmt.Sprintf("%s/Catalogs", c.baseURL)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get catalogs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var catalogResp CatalogResponse
	if err := json.NewDecoder(resp.Body).Decode(&catalogResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal catalogs: %w", err)
	}

	return catalogResp.Value, nil
}

// GetDatasetsWithQuery retrieves datasets with OData query options.
func (c *CBSClient) GetDatasetsWithQuery(catalog string, queryOptions map[string]string) ([]Dataset, int, error) {
	baseURL := fmt.Sprintf("%s/%s/Datasets", c.baseURL, catalog)

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	for key, value := range queryOptions {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get datasets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var datasetResp DatasetResponse
	if err := json.NewDecoder(resp.Body).Decode(&datasetResp); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal datasets: %w", err)
	}

	totalCount := len(datasetResp.Value)
	if datasetResp.ODataCount != nil {
		totalCount = *datasetResp.ODataCount
	}

	return datasetResp.Value, totalCount, nil
}

// GetDimensions retrieves all dimensions for a dataset.
func (c *CBSClient) GetDimensions(catalog, identifier string) ([]Dimension, error) {
	url := fmt.Sprintf("%s/%s/%s/Dimensions", c.baseURL, catalog, identifier)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get dimensions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var dimensionResp DimensionResponse
	if err := json.NewDecoder(resp.Body).Decode(&dimensionResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dimensions: %w", err)
	}

	return dimensionResp.Value, nil
}

// GetObservations retrieves observations from a dataset with optional filters.
func (c *CBSClient) GetObservations(catalog, dataset string, filters map[string]string) ([]map[string]any, error) {
	queryOptions := make(map[string]string)

	// Build filter string from filters map
	if len(filters) > 0 {
		var filterParts []string
		for key, value := range filters {
			filterParts = append(filterParts, fmt.Sprintf("%s eq '%s'", key, value))
		}
		queryOptions["$filter"] = strings.Join(filterParts, " and ")
	}

	path := fmt.Sprintf("/%s/%s/Observations", catalog, dataset)
	result, err := c.ExecuteQuery(path, queryOptions)
	if err != nil {
		return nil, err
	}

	// Extract observations from response
	observations, ok := result["value"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Convert to slice of maps
	var observationMaps []map[string]any
	for _, obs := range observations {
		if obsMap, ok := obs.(map[string]any); ok {
			observationMaps = append(observationMaps, obsMap)
		}
	}

	return observationMaps, nil
}

// GetMetadata retrieves the metadata document for the OData service.
func (c *CBSClient) GetMetadata() (string, error) {
	url := c.baseURL + "/$metadata"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/xml")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read metadata: %w", err)
	}

	return string(data), nil
}

// GetDimensionValues retrieves all values for a specific dimension.
func (c *CBSClient) GetDimensionValues(catalog, dataset, dimension string, queryOptions map[string]string) ([]map[string]any, error) {
	baseURL := fmt.Sprintf("%s/%s/%s/DimensionValues", c.baseURL, catalog, dataset)

	// Build URL with query parameters
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add dimension filter
	filterQuery := fmt.Sprintf("Dimension eq '%s'", dimension)

	// Add OData query parameters
	q := u.Query()
	q.Set("$filter", filterQuery)
	for key, value := range queryOptions {
		if key != "filter" && key != "$filter" {
			if strings.HasPrefix(key, "$") {
				q.Set(key, value)
			} else {
				q.Set("$"+key, value)
			}
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension values: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal dimension values: %w", err)
	}

	// Extract values from response
	values, ok := result["value"].([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format")
	}

	// Convert to slice of maps
	var valuesMaps []map[string]any
	for _, val := range values {
		if valMap, ok := val.(map[string]any); ok {
			valuesMaps = append(valuesMaps, valMap)
		}
	}

	return valuesMaps, nil
}

// ExecuteQuery executes an arbitrary OData query.
func (c *CBSClient) ExecuteQuery(path string, queryOptions map[string]string) (map[string]any, error) {
	baseURL := c.baseURL + path

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	for key, value := range queryOptions {
		if strings.HasPrefix(key, "$") {
			q.Set(key, value)
		} else {
			q.Set("$"+key, value)
		}
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal query results: %w", err)
	}

	return result, nil
}

// FormatDatasets formats datasets as a concise list.
func FormatDatasets(datasets []Dataset, totalCount int, skip int) string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("Found %d datasets (showing %d):\n\n", totalCount, len(datasets)))
	
	for i, dataset := range datasets {
		builder.WriteString(fmt.Sprintf("%d. **%s** (ID: `%s`)\n", skip+i+1, dataset.Title, dataset.Identifier))
		builder.WriteString(fmt.Sprintf("   - Modified: %s\n", dataset.Modified.Format("2006-01-02")))
		builder.WriteString(fmt.Sprintf("   - Status: %s\n", dataset.Status))
		builder.WriteString(fmt.Sprintf("   - Observations: %d\n", dataset.ObservationCount))
		builder.WriteString("\n")
	}
	
	return builder.String()
}

// FormatDimensions formats dimensions as a list.
func FormatDimensions(dimensions []Dimension) string {
	var builder strings.Builder
	
	builder.WriteString(fmt.Sprintf("Found %d dimensions:\n\n", len(dimensions)))
	
	for i, dim := range dimensions {
		builder.WriteString(fmt.Sprintf("%d. **%s** (ID: `%s`)\n", i+1, dim.Title, dim.Identifier))
		builder.WriteString(fmt.Sprintf("   - Kind: %s\n", dim.Kind))
		if dim.Description != "" {
			builder.WriteString(fmt.Sprintf("   - Description: %s\n", dim.Description))
		}
		builder.WriteString("\n")
	}
	
	return builder.String()
}

// FormatObservations formats observations as JSON.
func FormatObservations(data map[string]any, limit int) (string, error) {
	// Extract values array
	values, ok := data["value"].([]any)
	if !ok {
		return "", fmt.Errorf("unexpected response format")
	}
	
	// Limit results
	if limit > 0 && len(values) > limit {
		values = values[:limit]
	}
	
	// Format as JSON
	jsonData, err := json.MarshalIndent(map[string]any{
		"count": len(values),
		"data":  values,
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format observations: %w", err)
	}
	
	return string(jsonData), nil
}

// BuildQueryOptions builds OData query options from parameters.
func BuildQueryOptions(params map[string]string) map[string]string {
	options := make(map[string]string)
	
	for key, value := range params {
		if value != "" {
			options[key] = value
		}
	}
	
	return options
}

// ParseIntParam parses an integer parameter.
func ParseIntParam(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	
	return intValue
}
