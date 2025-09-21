package pihole

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAssertCNAME(t *testing.T, c *Client, expected *CNAMERecord, assertErr error) {
	actual, err := c.LocalCNAME.Get(context.TODO(), expected.Domain)
	if assertErr != nil {
		assert.ErrorAs(t, err, assertErr)
		return
	}

	require.NoError(t, err)

	assert.Equal(t, expected.Domain, actual.Domain)
	assert.Equal(t, expected.Target, actual.Target)
	assert.Equal(t, expected.HasTTL, actual.HasTTL)
	if expected.HasTTL {
		assert.Equal(t, expected.TTL, actual.TTL)
	}
}

func cleanupCNAME(t *testing.T, c *Client, domain string) {
	if err := c.LocalCNAME.Delete(context.TODO(), domain); err != nil {
		log.Printf("Failed to clean up CNAME record: %s\n", domain)
	}
}

func TestLocalCNAME(t *testing.T) {
	t.Run("Test create a CNAME record", func(t *testing.T) {
		isAcceptance(t)

		c := newTestClient(t)
		defer cleanupTestClient(c)

		domain := fmt.Sprintf("test.%s", randomID())

		record, err := c.LocalCNAME.Create(context.Background(), domain, "domain.com")
		require.NoError(t, err)

		defer cleanupCNAME(t, c, domain)

		testAssertCNAME(t, c, record, nil)
		testAssertCNAME(t, c, &CNAMERecord{
			Domain: record.Domain,
			Target: "domain.com",
		}, nil)
	})

	t.Run("Test delete a CNAME record", func(t *testing.T) {
		isAcceptance(t)

		c := newTestClient(t)
		defer cleanupTestClient(c)

		ctx := context.Background()

		domain := fmt.Sprintf("test.%s", randomID())

		record, err := c.LocalCNAME.Create(ctx, domain, "domain.com")
		require.NoError(t, err)
		defer cleanupCNAME(t, c, record.Domain)

		err = c.LocalCNAME.Delete(ctx, domain)
		require.NoError(t, err)

		_, err = c.LocalCNAME.Get(ctx, domain)
		assert.ErrorIs(t, err, ErrorLocalCNAMENotFound)
	})
}

func TestCNAMERecordListResponse_toCNAMERecordList(t *testing.T) {
	t.Run("parses records with optional TTL", func(t *testing.T) {
		resp := cnameRecordListResponse{
			Config: cnameRecordConfigListResponse{
				DNS: cnameRecordDNSListResponse{
					CNAMERecords: []string{"example.com , target.test , 3600"},
				},
			},
		}

		records, err := resp.toCNAMERecordList()
		require.NoError(t, err)
		require.Len(t, records, 1)
		assert.Equal(t, "example.com", records[0].Domain)
		assert.Equal(t, "target.test", records[0].Target)
		assert.True(t, records[0].HasTTL)
		assert.Equal(t, 3600, records[0].TTL)
	})

	t.Run("returns an error for malformed records", func(t *testing.T) {
		resp := cnameRecordListResponse{
			Config: cnameRecordConfigListResponse{
				DNS: cnameRecordDNSListResponse{
					CNAMERecords: []string{"example.com"},
				},
			},
		}

		_, err := resp.toCNAMERecordList()
		require.Error(t, err)
	})

	t.Run("returns an error when TTL is invalid", func(t *testing.T) {
		resp := cnameRecordListResponse{
			Config: cnameRecordConfigListResponse{
				DNS: cnameRecordDNSListResponse{
					CNAMERecords: []string{"example.com,target.test,not-a-number"},
				},
			},
		}

		_, err := resp.toCNAMERecordList()
		require.Error(t, err)
	})
}

func TestLocalCNAME_CreateRecordWithTTLAndDeletePreservesTuple(t *testing.T) {
	rawTuple := "example.com,target.test,3600"
	expectedPath := "/api/config/dns/cnameRecords/" + url.PathEscape(rawTuple)

	var (
		receivedPUTPath    string
		receivedDeletePath string
	)

	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodPut && strings.HasPrefix(req.URL.Path, "/api/config/dns/cnameRecords/"):
			receivedPUTPath = req.URL.EscapedPath()
			if receivedPUTPath != expectedPath {
				t.Fatalf("unexpected PUT path: %s", req.URL.Path)
			}
			return newHTTPResponse(http.StatusCreated, ``), nil
		case req.Method == http.MethodGet && req.URL.Path == "/api/config/dns/cnameRecords":
			return newHTTPResponse(http.StatusOK, fmt.Sprintf(`{"config":{"dns":{"cnameRecords":["%s"]}}}`, rawTuple)), nil
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/api/config/dns/cnameRecords/"):
			receivedDeletePath = req.URL.EscapedPath()
			if receivedDeletePath != expectedPath {
				t.Fatalf("unexpected DELETE path: %s", req.URL.Path)
			}
			return newHTTPResponse(http.StatusNoContent, ``), nil
		default:
			return newHTTPResponse(http.StatusNotFound, ``), nil
		}
	})}

	client, err := New(Config{
		BaseURL:    "http://pi.test",
		SessionID:  "test",
		HttpClient: httpClient,
	})
	require.NoError(t, err)

	ctx := context.Background()
	record, err := client.LocalCNAME.CreateRecord(ctx, &CNAMERecord{Domain: "example.com", Target: "target.test", TTL: 3600, HasTTL: true})
	require.NoError(t, err)
	assert.True(t, record.HasTTL)
	assert.Equal(t, 3600, record.TTL)

	require.NoError(t, client.LocalCNAME.Delete(ctx, "example.com"))

	assert.Equal(t, expectedPath, receivedPUTPath)
	assert.Equal(t, expectedPath, receivedDeletePath)
}

func TestLocalCNAME_DeleteReturnsAPIError(t *testing.T) {
	isUnit(t)

	const rawTuple = "example.com,target.test,3600"

	httpClientError := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/api/config/dns/cnameRecords":
			return newHTTPResponse(http.StatusOK, fmt.Sprintf(`{"config":{"dns":{"cnameRecords":["%s"]}}}`, rawTuple)), nil
		case req.Method == http.MethodDelete && strings.HasPrefix(req.URL.Path, "/api/config/dns/cnameRecords/"):
			return newHTTPResponse(http.StatusNotFound, `{"error":{"key":"not_found","message":"missing","hint":null}}`), nil
		default:
			return newHTTPResponse(http.StatusNotFound, ``), nil
		}
	})}

	client, err := New(Config{
		BaseURL:    "http://pi.test",
		SessionID:  "test",
		HttpClient: httpClientError,
	})
	require.NoError(t, err)

	err = client.LocalCNAME.Delete(context.Background(), "example.com")
	var apiErr *CNAMEAPIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.StatusCode)
	assert.Equal(t, "not_found", apiErr.Key)
	assert.Equal(t, "missing", apiErr.Message)
}
