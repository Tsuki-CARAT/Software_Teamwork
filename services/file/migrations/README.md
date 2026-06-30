# File Service Migrations

File metadata persistence uses forward-only goose migrations in this directory. The first table stores base file metadata only:

- internal file id
- display filename
- content type
- size bytes
- checksum
- server-generated object key
- created, deleted, delete-requested, purged, and updated timestamps
- sanitized cleanup failure code/message

Do not store raw file contents in PostgreSQL and do not expose object keys through API responses.

Do not store knowledge-base IDs, knowledge document IDs, report IDs, template IDs, material IDs, report file IDs, business tags, processing status, or ACLs here. Those belong to the owner service that references the base file object.
