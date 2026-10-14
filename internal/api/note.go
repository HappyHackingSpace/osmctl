package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/paulmach/osm"
)

// NoteQuery searches notes by text, area, or both. ClosedDays follows the API:
// 0 means open notes only, -1 means every note regardless of age.
type NoteQuery struct {
	Text       string
	Bounds     *osm.Bounds
	Limit      int
	ClosedDays int
}

func (c *Client) Note(ctx context.Context, id osm.NoteID) (*osm.Note, error) {
	return c.noteRequest(ctx, "GET", fmt.Sprintf("/notes/%d", id), nil)
}

func (c *Client) Notes(ctx context.Context, q NoteQuery) (osm.Notes, error) {
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
	var o osm.OSM
	if err := c.get(ctx, path+"?"+v.Encode(), &o); err != nil {
		return nil, err
	}
	return o.Notes, nil
}

func (c *Client) CreateNote(ctx context.Context, lat, lon float64, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, "POST", "/notes", url.Values{
		"lat":  {strconv.FormatFloat(lat, 'f', -1, 64)},
		"lon":  {strconv.FormatFloat(lon, 'f', -1, 64)},
		"text": {text},
	})
}

func (c *Client) CommentNote(ctx context.Context, id osm.NoteID, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, "POST", fmt.Sprintf("/notes/%d/comment", id), url.Values{"text": {text}})
}

func (c *Client) CloseNote(ctx context.Context, id osm.NoteID, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, "POST", fmt.Sprintf("/notes/%d/close", id), optionalText(text))
}

func (c *Client) ReopenNote(ctx context.Context, id osm.NoteID, text string) (*osm.Note, error) {
	return c.noteRequest(ctx, "POST", fmt.Sprintf("/notes/%d/reopen", id), optionalText(text))
}

func optionalText(text string) url.Values {
	if text == "" {
		return url.Values{}
	}
	return url.Values{"text": {text}}
}

func (c *Client) noteRequest(ctx context.Context, method, path string, form url.Values) (*osm.Note, error) {
	var o osm.OSM
	var err error
	if method == "GET" {
		err = c.get(ctx, path, &o)
	} else {
		err = c.postForm(ctx, path, form, &o)
	}
	if err != nil {
		return nil, err
	}
	if len(o.Notes) == 0 {
		return nil, fmt.Errorf("%s %s: response contained no note", method, path)
	}
	return o.Notes[0], nil
}
