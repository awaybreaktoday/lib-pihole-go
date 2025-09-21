package pihole

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type LocalCNAME interface {
	// List all CNAME records.
	List(ctx context.Context) (CNAMERecordList, error)

	// Create a CNAME record.
	Create(ctx context.Context, domain string, target string) (*CNAMERecord, error)

	// CreateRecord creates a CNAME record using the provided record definition.
	CreateRecord(ctx context.Context, record *CNAMERecord) (*CNAMERecord, error)

	// Get a CNAME record by its domain.
	Get(ctx context.Context, domain string) (*CNAMERecord, error)

	// Delete a CNAME record by its domain.
	Delete(ctx context.Context, domain string) error
}

var (
	ErrorLocalCNAMENotFound = errors.New("local CNAME record not found")
)

type localCNAME struct {
	client *Client
}

type CNAMERecord struct {
	Domain string
	Target string
	TTL    int
	HasTTL bool
	raw    string
}

type cnameRecordListResponse struct {
	Config cnameRecordConfigListResponse `json:"config"`
}

type cnameRecordConfigListResponse struct {
	DNS cnameRecordDNSListResponse `json:"dns"`
}

type cnameRecordDNSListResponse struct {
	CNAMERecords []string `json:"cnameRecords"`
}

func (res cnameRecordListResponse) toCNAMERecordList() (CNAMERecordList, error) {
	list := make(CNAMERecordList, 0, len(res.Config.DNS.CNAMERecords))

	for _, entry := range res.Config.DNS.CNAMERecords {
		record, err := parseCNAMERecord(entry)
		if err != nil {
			return nil, err
		}

		list = append(list, record)
	}

	return list, nil
}

func parseCNAMERecord(raw string) (CNAMERecord, error) {
	entry := strings.Split(raw, ",")
	if len(entry) < 2 || len(entry) > 3 {
		return CNAMERecord{}, fmt.Errorf("invalid CNAME record: %q", raw)
	}

	record := CNAMERecord{
		Domain: strings.TrimSpace(entry[0]),
		Target: strings.TrimSpace(entry[1]),
		raw:    strings.TrimSpace(raw),
	}

	if len(entry) == 3 {
		ttlStr := strings.TrimSpace(entry[2])
		if ttlStr != "" {
			ttl, err := strconv.Atoi(ttlStr)
			if err != nil {
				return CNAMERecord{}, fmt.Errorf("invalid TTL in CNAME record %q: %w", raw, err)
			}
			record.TTL = ttl
			record.HasTTL = true
		}
	}

	return record, nil
}

type CNAMERecordList []CNAMERecord

// List returns all CNAME records
func (cname localCNAME) List(ctx context.Context) (CNAMERecordList, error) {
	res, err := cname.client.Get(ctx, "/api/config/dns/cnameRecords")
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	var resList *cnameRecordListResponse
	if err := json.NewDecoder(res.Body).Decode(&resList); err != nil {
		return nil, fmt.Errorf("failed to parse custom CNAME list body: %w", err)
	}

	records, err := resList.toCNAMERecordList()
	if err != nil {
		return nil, fmt.Errorf("failed to parse custom CNAME list body: %w", err)
	}

	return records, nil
}

// Create creates a CNAME record
func (cname localCNAME) Create(ctx context.Context, domain string, target string) (*CNAMERecord, error) {
	return cname.CreateRecord(ctx, &CNAMERecord{Domain: domain, Target: target})
}

// CreateRecord creates a CNAME record using the provided record definition.
func (cname localCNAME) CreateRecord(ctx context.Context, record *CNAMERecord) (*CNAMERecord, error) {
	value := encodeCNAMERecord(record)
	res, err := cname.client.Put(ctx, fmt.Sprintf("/api/config/dns/cnameRecords/%s", value), nil)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		return nil, newCNAMEAPIError(res.StatusCode, b)
	}

	return cname.Get(ctx, record.Domain)
}

// Get returns a CNAME record by the passed domain
func (cname localCNAME) Get(ctx context.Context, domain string) (*CNAMERecord, error) {
	list, err := cname.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch custom CNAME records: %w", err)
	}

	for _, record := range list {
		if strings.EqualFold(record.Domain, domain) {
			return &record, nil
		}
	}

	return nil, fmt.Errorf("%w: %s", ErrorLocalCNAMENotFound, domain)
}

// Delete removes a CNAME record by domain
func (cname localCNAME) Delete(ctx context.Context, domain string) error {
	record, err := cname.Get(ctx, domain)
	if err != nil {
		if errors.Is(err, ErrorLocalCNAMENotFound) {
			return nil
		}
		return fmt.Errorf("failed looking up CNAME record %s for deletion: %w", domain, err)
	}

	value := encodeCNAMERecord(record)

	res, err := cname.client.Delete(ctx, fmt.Sprintf("/api/config/dns/cnameRecords/%s", value))
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(res.Body)
		return newCNAMEAPIError(res.StatusCode, b)
	}

	return nil
}

func encodeCNAMERecord(record *CNAMERecord) string {
	if record == nil {
		return ""
	}

	if record.raw != "" && record.Domain != "" {
		return escapeCNAMEValue(record.raw)
	}

	parts := []string{strings.TrimSpace(record.Domain), strings.TrimSpace(record.Target)}
	if record.HasTTL {
		parts = append(parts, strconv.Itoa(record.TTL))
	}

	raw := strings.Join(parts, ",")
	return escapeCNAMEValue(raw)
}

func escapeCNAMEValue(value string) string {
	escaped := url.PathEscape(value)
	return strings.ReplaceAll(escaped, ",", "%2C")
}
