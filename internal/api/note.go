package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// NoteQuery searches notes by text, area, or both. ClosedDays follows the API:
// 0 means open notes only, -1 means every note regardless of age.
type NoteQuery struct {
	Text       string
	Bounds     *osm.Bounds
	Limit      int
	ClosedDays int
}

func (c *Client) Note(ctx context.Context, id int64) (*osm.Note, error) {
	return c.noteRequest(ctx, http.MethodGet, fmt.Sprintf("/notes/%d", id), nil)
}

func (c *Client) Notes(ctx context.Context, q NoteQuery) ([]*osm.Note, error) {
	v := url.Values{}
	v.Set("closed", strconv.Itoa(q.ClosedDays))
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	path := "/notes"
	if q.Text != "" {
		path = "/notes/search"
		v.Set("q", q.Text)
	}
	if q.Bounds != nil {
		v.Set("bbox", bboxParam(q.Bounds))
	}
	d, err := c.getData(ctx, path+"?"+v.Encode())
	if err != nil {
		return nil, err
	}
	return d.Notes, nil
}

func (c *Client) CreateNote(ctx context.Context, lat, lon float64, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, http.MethodPost, "/notes", url.Values{
		"lat":  {osm.FormatFloat(lat)},
		"lon":  {osm.FormatFloat(lon)},
		"text": {text},
	})
}

func (c *Client) CommentNote(ctx context.Context, id int64, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, http.MethodPost, fmt.Sprintf("/notes/%d/comment", id), url.Values{"text": {text}})
}

func (c *Client) CloseNote(ctx context.Context, id int64, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, http.MethodPost, fmt.Sprintf("/notes/%d/close", id), optionalText(text))
}

func (c *Client) ReopenNote(ctx context.Context, id int64, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, http.MethodPost, fmt.Sprintf("/notes/%d/reopen", id), optionalText(text))
}

func optionalText(text string) url.Values {
	if text == "" {
		return url.Values{}
	}
	return url.Values{"text": {text}}
}

func (c *Client) noteRequest(ctx context.Context, method, path string, form url.Values) (*osm.Note, error) {
	var d *osm.Data
	read := func(r io.Reader) (err error) {
		d, err = osm.ReadXML(r)
		return err
	}
	var err error
	if method == http.MethodGet {
		err = c.do(ctx, method, path, nil, "", read)
	} else {
		err = c.postForm(ctx, path, form, read)
	}
	if err != nil {
		return nil, err
	}
	if len(d.Notes) == 0 {
		return nil, fmt.Errorf("%s %s: response contained no note", method, path)
	}
	return d.Notes[0], nil
}
