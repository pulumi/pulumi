// Copyright 2025, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package postgres implements a blob.Bucket storage backend using PostgreSQL.
//
// SECURITY NOTE - SQL Injection Prevention:
// This implementation uses dynamic table names in SQL queries, which could appear
// to be vulnerable to SQL injection. However, this is safe because:
//
//  1. Table names come from the PostgreSQL connection string configuration,
//     which is controlled by system administrators/developers, not end users
//  2. The connection string is part of the backend configuration, not user input
//  3. All actual user-provided data (keys, blob data) are properly parameterized
//     using prepared statement placeholders ($1, $2, etc.)
//  4. This pattern allows for configurable table schemas while maintaining security
//
// The //nolint:gosec comments acknowledge that static analysis tools may flag
// these patterns, but they are intentionally safe in this controlled context.
package postgres

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	_ "embed"

	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"

	// Import the PostgreSQL driver
	_ "github.com/lib/pq"
)

// blobData represents the JSON structure for storing blob data in PostgreSQL
type blobData struct {
	Data string `json:"data"` // base64 encoded binary data
}

// Bucket implements blob.Bucket storage using PostgreSQL.
type Bucket struct {
	db        *sql.DB
	tableName string
	bucket    *blob.Bucket
}

//go:embed schema.sql
var tableSchema string

// NewPostgresBucket creates a new Bucket.
func NewPostgresBucket(ctx context.Context, connString string) (*Bucket, error) {
	u, err := url.Parse(connString)
	if err != nil {
		return nil, fmt.Errorf("invalid PostgreSQL connection string: %w", err)
	}

	// Extract table name from query parameters or use default
	// SECURITY NOTE: The table name comes from the connection string configuration,
	// which is controlled by system administrators/developers, not end users.
	// This is not user input and therefore safe from SQL injection.
	q := u.Query()
	tableName := q.Get("table")
	if tableName == "" {
		tableName = "pulumi_state"
	}
	q.Del("table") // Remove it from connection string
	u.RawQuery = q.Encode()

	// Connect to database
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Set connection pool parameters based on CPU count
	db.SetMaxOpenConns(max(runtime.GOMAXPROCS(0)*2, 2))
	db.SetMaxIdleConns(max(runtime.GOMAXPROCS(0), 1))
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	// Create table if it doesn't exist
	// SECURITY NOTE: tableName is from connection string config, not user input - safe from SQL injection
	createTableSQL := fmt.Sprintf(tableSchema, tableName, tableName, tableName)
	if _, err := db.ExecContext(ctx, createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	bucket := &Bucket{
		db:        db,
		tableName: tableName,
	}

	// Create the driver.Bucket implementation
	drv := &postgresBucketDriver{bucket: bucket}

	// Wrap with blob.Bucket
	bucket.bucket = blob.NewBucket(drv)
	return bucket, nil
}

// Bucket returns the blob.Bucket.
func (b *Bucket) Bucket() *blob.Bucket {
	return b.bucket
}

// Close closes the bucket.
func (b *Bucket) Close() error {
	err := b.bucket.Close()
	if b.db != nil {
		dbErr := b.db.Close()
		if err == nil {
			err = dbErr
		}
		b.db = nil
	}
	return err
}

// postgresBucketDriver implements driver.Bucket.
type postgresBucketDriver struct {
	bucket *Bucket
}

// As implements driver.Bucket.As.
func (d *postgresBucketDriver) As(i interface{}) bool {
	p, ok := i.(**Bucket)
	if !ok {
		return false
	}
	*p = d.bucket
	return true
}

// ErrorAs implements driver.Bucket.ErrorAs.
func (d *postgresBucketDriver) ErrorAs(err error, i interface{}) bool {
	return false
}

// ErrorCode implements driver.Bucket.ErrorCode.
func (d *postgresBucketDriver) ErrorCode(err error) gcerrors.ErrorCode {
	if errors.Is(err, sql.ErrNoRows) {
		return gcerrors.NotFound
	}
	return gcerrors.Unknown
}

// Copy implements driver.Bucket.Copy.
func (d *postgresBucketDriver) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	// Read the source data
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := fmt.Sprintf("SELECT data FROM %s WHERE key = $1", d.bucket.tableName) //nolint:gosec
	var dataJSON string
	err := d.bucket.db.QueryRowContext(ctx, query, srcKey).Scan(&dataJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("source key not found: %w", err)
		}
		return err
	}

	// Write to the destination key
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	insertQuery := fmt.Sprintf( //nolint:gosec
		"INSERT INTO %s (key, data) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = now()",
		d.bucket.tableName,
	)
	_, err = d.bucket.db.ExecContext(ctx, insertQuery, dstKey, dataJSON)
	return err
}

// ListPaged implements driver.Bucket.ListPaged.
func (d *postgresBucketDriver) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {
	// The SQL query to list blob keys
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := "SELECT key FROM " + d.bucket.tableName //nolint:gosec
	args := []interface{}{}

	// Add conditions to filter by prefix
	if opts.Prefix != "" {
		query += " WHERE key LIKE $1"
		args = append(args, opts.Prefix+"%")
	}

	// Add sorting
	query += " ORDER BY key"

	// Add pagination
	if opts.PageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.PageSize)
	}
	if len(opts.PageToken) > 0 {
		query += " OFFSET $" + strconv.Itoa(len(args)+1)
		offset, err := strconv.Atoi(string(opts.PageToken))
		if err != nil {
			return nil, err
		}
		args = append(args, offset)
	}

	// Execute the query
	rows, err := d.bucket.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Process the results
	page := &driver.ListPage{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}

		// Determine if this is a "directory" by checking delimiter
		if opts.Delimiter != "" {
			// If the key contains the delimiter, we need to create a "directory" object
			keyAfterPrefix := strings.TrimPrefix(key, opts.Prefix)
			delimiterIndex := strings.Index(keyAfterPrefix, opts.Delimiter)
			if delimiterIndex >= 0 {
				dirPrefix := key[:len(opts.Prefix)+delimiterIndex+1]
				// Check if we already added this directory
				found := false
				for _, obj := range page.Objects {
					if obj.Key == dirPrefix {
						found = true
						break
					}
				}

				if !found {
					page.Objects = append(page.Objects, &driver.ListObject{
						Key:     dirPrefix,
						ModTime: time.Time{},
						Size:    0,
						IsDir:   true,
					})
				}
				continue
			}
		}

		// Get metadata for the object
		var updatedAt time.Time
		var size int64
		// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
		metaQuery := fmt.Sprintf( //nolint:gosec
			"SELECT updated_at, octet_length((data->>'data')::text) / 4 * 3 FROM %s WHERE key = $1",
			d.bucket.tableName,
		)
		err := d.bucket.db.QueryRowContext(ctx, metaQuery, key).Scan(&updatedAt, &size)
		if err != nil {
			return nil, err
		}

		page.Objects = append(page.Objects, &driver.ListObject{
			Key:     key,
			ModTime: updatedAt,
			Size:    size,
			IsDir:   false,
		})
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// If we retrieved page size + 1 items, set NextPageToken
	if opts.PageSize > 0 && len(page.Objects) > opts.PageSize {
		// Remove the extra item
		page.Objects = page.Objects[:opts.PageSize]
		// Generate next page token
		offsetStr := strconv.Itoa(opts.PageSize)
		page.NextPageToken = []byte(offsetStr)
	}

	return page, nil
}

// Attributes implements driver.Bucket.Attributes.
func (d *postgresBucketDriver) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := fmt.Sprintf( //nolint:gosec
		"SELECT updated_at, octet_length((data->>'data')::text) / 4 * 3 FROM %s WHERE key = $1",
		d.bucket.tableName,
	)
	var updatedAt time.Time
	var size int64
	err := d.bucket.db.QueryRowContext(ctx, query, key).Scan(&updatedAt, &size)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key not found: %w", err)
		}
		return nil, err
	}

	return &driver.Attributes{
		CacheControl:       "",
		ContentDisposition: "",
		ContentEncoding:    "",
		ContentLanguage:    "",
		ContentType:        "application/octet-stream",
		Metadata:           nil,
		ModTime:            updatedAt,
		Size:               size,
		MD5:                nil,
		ETag:               "",
	}, nil
}

// NewRangeReader implements driver.Bucket.NewRangeReader.
func (d *postgresBucketDriver) NewRangeReader(
	ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions,
) (driver.Reader, error) {
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := fmt.Sprintf("SELECT data FROM %s WHERE key = $1", d.bucket.tableName) //nolint:gosec
	var dataJSON string
	err := d.bucket.db.QueryRowContext(ctx, query, key).Scan(&dataJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key not found: %w", err)
		}
		return nil, err
	}

	// Parse the JSON and decode the base64 data
	var blobData blobData
	if err := json.Unmarshal([]byte(dataJSON), &blobData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON data: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(blobData.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Apply offset and length
	size := int64(len(data))
	if offset < 0 || offset > size {
		return nil, fmt.Errorf("invalid offset: %d, size: %d", offset, size)
	}

	if length < 0 {
		length = size - offset
	} else if offset+length > size {
		length = size - offset
	}

	// Create an io.Reader from the data
	r := io.NewSectionReader(strings.NewReader(string(data)), offset, length)
	return &postgresReader{
		r:        r,
		size:     length,
		modTime:  time.Now(),
		metadata: nil,
	}, nil
}

// NewTypedWriter implements driver.Bucket.NewTypedWriter.
func (d *postgresBucketDriver) NewTypedWriter(
	ctx context.Context, key string, contentType string, opts *driver.WriterOptions,
) (driver.Writer, error) {
	return &postgresWriter{
		ctx:         ctx,
		bucket:      d.bucket,
		key:         key,
		contentType: contentType,
		opts:        opts,
	}, nil
}

// Delete implements driver.Bucket.Delete.
func (d *postgresBucketDriver) Delete(ctx context.Context, key string) error {
	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := fmt.Sprintf("DELETE FROM %s WHERE key = $1", d.bucket.tableName) //nolint:gosec
	result, err := d.bucket.db.ExecContext(ctx, query, key)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("key not found: %s", key)
	}

	return nil
}

// SignedURL implements driver.Bucket.SignedURL.
func (d *postgresBucketDriver) SignedURL(
	ctx context.Context, key string, opts *driver.SignedURLOptions,
) (string, error) {
	// PostgreSQL doesn't support pre-signed URLs
	return "", errors.New("signed URLs not supported with PostgreSQL backend")
}

// Close implements driver.Bucket.Close.
func (d *postgresBucketDriver) Close() error {
	return nil
}

// postgresReader implements driver.Reader for PostgreSQL.
type postgresReader struct {
	r        io.ReadSeeker
	size     int64
	modTime  time.Time
	metadata map[string]string
}

// Read implements io.Reader.
func (r *postgresReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

// Seek implements io.Seeker.
func (r *postgresReader) Seek(offset int64, whence int) (int64, error) {
	return r.r.Seek(offset, whence)
}

// Close implements io.Closer.
func (r *postgresReader) Close() error {
	return nil
}

// Attributes implements driver.Reader.Attributes.
func (r *postgresReader) Attributes() *driver.ReaderAttributes {
	return &driver.ReaderAttributes{
		ContentType: "application/octet-stream",
		ModTime:     r.modTime,
		Size:        r.size,
	}
}

// As implements driver.Reader.As.
func (r *postgresReader) As(i interface{}) bool {
	return false
}

// postgresWriter implements driver.Writer for PostgreSQL.
type postgresWriter struct {
	ctx         context.Context
	bucket      *Bucket
	key         string
	contentType string
	opts        *driver.WriterOptions
	buf         []byte
}

// Write implements io.Writer.
func (w *postgresWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// Close implements io.Closer.
func (w *postgresWriter) Close() error {
	// Encode the binary data as base64 and wrap in JSON
	encodedData := base64.StdEncoding.EncodeToString(w.buf)
	blobData := blobData{Data: encodedData}
	jsonData, err := json.Marshal(blobData)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON data: %w", err)
	}

	// SECURITY: tableName is from connection string config, not user input - safe from SQL injection
	query := fmt.Sprintf( //nolint:gosec
		"INSERT INTO %s (key, data) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = now()",
		w.bucket.tableName,
	)
	_, err = w.bucket.db.ExecContext(w.ctx, query, w.key, string(jsonData))
	return err
}

// As implements driver.Writer.As.
func (w *postgresWriter) As(i interface{}) bool {
	return false
}
