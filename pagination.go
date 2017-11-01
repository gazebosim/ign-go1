package ign

import (
  "fmt"
  "net/http"
  "net/url"
  "strconv"
  "github.com/jinzhu/gorm"
)

const (
  defaultPageSize = 20
  maxPageSize = 100
  defaultPageNumber = 1

  pageArgName = "page"
  perPageArgName = "per_page"
)
//////////////////////////////////////

// Pagination module is used to perform GORM 'Find' queries in
// a paginated way.
// The typical usage is the following:
// 1) Create a PaginationRequest from que HTTP request. This means
// reading 'page' and 'per_page' arguments sent by the user in the
// URL query.
// eg. pagRequest := NewPaginationRequest(r)
// 2) Create your GORM Query and the paginate it:
// eg. q := db.Model(&Model{})
// pagResult := PaginateQuery(q, result, pagRequest)
// 3) Write the prev and next headers in the output response
// WritePaginationHeaders(pagResult, w, r)

//////////////////////////////////////

// PaginationRequest represents the pagination values requested
// in the URL query (eg. ?page=2&per_page=10)
type PaginationRequest struct {
  // Flag that indicates if the request included a "page" argument.
  PageRequested bool
  // The requested page number (value >= 1)
  Page int64
  // The requested number of items per page.
  PerPage int64
  // The original request URL
  URL string
}

// NewPaginationRequest creates a new PaginationRequest from the given http request.
func NewPaginationRequest(r *http.Request) (*PaginationRequest, *ErrMsg) {
  pageRequest := PaginationRequest{
    PageRequested: false,
    Page: defaultPageNumber,
    PerPage: defaultPageSize,
    URL: r.URL.String(),
  }
  var err error

  // Parse request arguments

  // Process "page" argument
  pageStr := r.URL.Query().Get(pageArgName)
  if pageStr != "" {
    pageRequest.PageRequested = true
    pageRequest.Page, err = strconv.ParseInt(pageStr, 10, 64)
    if err != nil {
      return nil, NewErrorMessageWithArgs(ErrorInvalidPaginationRequest, err, []string{pageArgName})
    }
    if pageRequest.Page <= 0 {
      return nil, NewErrorMessageWithArgs(ErrorInvalidPaginationRequest, nil, []string{pageArgName})
    }
  }

  // Process "per_page" argument
  perPageStr := r.URL.Query().Get(perPageArgName)
  if perPageStr != "" {
    pageRequest.PerPage, err = strconv.ParseInt(perPageStr, 10, 64)
    if err != nil {
      return nil, NewErrorMessageWithArgs(ErrorInvalidPaginationRequest, err, []string{perPageArgName})
    }
    if pageRequest.PerPage <= 0 {
      return nil, NewErrorMessageWithArgs(ErrorInvalidPaginationRequest, err, []string{perPageArgName})
    }
    if pageRequest.PerPage > maxPageSize {
      pageRequest.PerPage = defaultPageSize
    }
  }
  return &pageRequest, nil
}

//////////////////////////////////////

// PaginationResult represents the actual pagination output.
type PaginationResult struct {
  // Page number
  Page int64
  // Page size
  PerPage int64
  // Original request' url
  URL string
  // Query "total" count (ie. this is NOT the "page" count)
  QueryCount int64
  // A page is considered "found" if it is within the range of valid pages,
  // OR if it is the first page and the DB query is empty. In this empty scenario,
  // we want to return status OK with zero elements, rather than a 404 status.
  PageFound bool
}

func newPaginationResult() PaginationResult {
  return PaginationResult{}
}

//////////////////////////////////////

func computeLastPage(page *PaginationResult) int64 {
  mod := page.QueryCount % page.PerPage
  lastPage := page.QueryCount / page.PerPage
  if mod > 0 {
    lastPage++
  }
  return lastPage
}

// PaginateQuery applies a pagination request to a GORM query and executes it.
// Param[in] q [gorm.DB] The query to be paginated
// Param[out] result [interface{}] The paginated list of items
// Param[in] p The pagination request
// Returns a PaginationResult describing the returned page.
func PaginateQuery(q *gorm.DB, result interface{}, p PaginationRequest) (*PaginationResult, error) {
  q = q.Limit(int(p.PerPage))
  q = q.Offset((Max(p.Page, 1) - 1) * p.PerPage)
  q = q.Find(result)
  if err := q.Error; err != nil {
    return nil, err
  }
  q = q.Limit(-1)
  q = q.Offset(-1)
  count := 0
  if err := q.Count(&count).Error; err != nil {
    return nil, err
  }

  r := newPaginationResult()
  r.Page = p.Page
  r.PerPage = p.PerPage
  r.URL = p.URL
  r.QueryCount = int64(count)

  lastPage := computeLastPage(&r)
  // A page is considered "found" if it is within the range of valid pages,
  // OR if it is the first page and the DB query is empty. In this empty scenario,
  // we want to return status OK with zero elements, rather than a 404 status.
  r.PageFound = r.Page <= lastPage || (r.Page == 1 && r.QueryCount == 0)

  return &r, nil
}

//////////////////////////////////////

// newLinkStr is a helper function to create a page link header string.
func newLinkStr(u *url.URL, page int64, name string) string {
  params := u.Query()
  params.Set(pageArgName, fmt.Sprint(page))
  u.RawQuery = params.Encode()
  return fmt.Sprintf("<%s>; rel=\"%s\"", u, name)
}

// WritePaginationHeaders writes the 'next', 'last', 'first', and 'prev' Link headers to the given
// ResponseWriter.
func WritePaginationHeaders(page PaginationResult, w http.ResponseWriter, r *http.Request) error {
  u , _ := url.Parse(page.URL)
  params := u.Query()
  params.Set(perPageArgName, fmt.Sprint(page.PerPage))

  lastPage := computeLastPage(&page)

  var links []string

  // Next and Last
  if page.Page < lastPage {
    links = append(links, newLinkStr(u, page.Page + 1, "next"))
    links = append(links, newLinkStr(u, lastPage, "last"))
  }

  // First and Prev
  if page.Page > 1 {
    links = append(links, newLinkStr(u, 1, "first"))
    prev := page.Page - 1
    if page.Page > lastPage {
      prev = lastPage
    }
    links = append(links, newLinkStr(u, prev, "prev"))
  }

  // Build the output Links header
  c := len(links)
  headerStr := ""
  for i, l := range links {
    headerStr += l
    if i+1 < c {
      headerStr += ", "
    }
  }
  w.Header().Set("Link", headerStr)
  w.Header().Set("X-Total-Count", fmt.Sprint(page.QueryCount))
  return nil
}
