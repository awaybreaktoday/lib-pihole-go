package pihole

import (
	"context"
	"fmt"
	"log"
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
