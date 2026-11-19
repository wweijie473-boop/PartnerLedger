CREATE TABLE pl_sheet_documents (
    id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
    organization_id BIGINT UNSIGNED NOT NULL,
    owner_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(120) NOT NULL,
    visibility ENUM('private', 'org_read', 'org_edit') NOT NULL,
    row_count INT UNSIGNED NOT NULL,
    columns_json JSON NOT NULL,
    revision BIGINT UNSIGNED NOT NULL,
    create_request CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    create_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    INDEX pl_sheet_org (organization_id, id),
    UNIQUE KEY pl_sheet_creation (organization_id, owner_id, create_request)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_sheet_cells (
    sheet_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    row_no INT UNSIGNED NOT NULL,
    col_no SMALLINT UNSIGNED NOT NULL,
    input VARCHAR(1000) NOT NULL,
    PRIMARY KEY (sheet_id, row_no, col_no),
    FOREIGN KEY (sheet_id) REFERENCES pl_sheet_documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_sheet_changes (
    sheet_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    actor_id BIGINT UNSIGNED NOT NULL,
    request_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash CHAR(64) CHARACTER SET ascii NOT NULL,
    revision BIGINT UNSIGNED NOT NULL,
    cell_count SMALLINT UNSIGNED NOT NULL,
    changed_at DATETIME(6) NOT NULL,
    PRIMARY KEY (sheet_id, actor_id, request_id),
    UNIQUE KEY pl_sheet_revision (sheet_id, revision),
    FOREIGN KEY (sheet_id) REFERENCES pl_sheet_documents(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
