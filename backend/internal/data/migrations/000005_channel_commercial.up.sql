CREATE TABLE pl_channel_quotes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    project_id BIGINT UNSIGNED NOT NULL,
    kind ENUM('channel', 'sales') NOT NULL,
    version INT UNSIGNED NOT NULL,
    status ENUM('active', 'superseded', 'confirmed', 'ordered') NOT NULL,
    channel_quote_id BIGINT UNSIGNED NULL,
    quote_lines JSON NOT NULL,
    total_cents BIGINT UNSIGNED NOT NULL,
    valid_until DATE NOT NULL,
    delivery_days INT UNSIGNED NOT NULL,
    note TEXT NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY pl_quote_version (project_id, kind, version),
    UNIQUE KEY pl_quote_project_id (project_id, id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id),
    FOREIGN KEY (project_id, channel_quote_id) REFERENCES pl_channel_quotes(project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_channel_orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    project_id BIGINT UNSIGNED NOT NULL,
    channel_quote_id BIGINT UNSIGNED NOT NULL,
    sales_quote_id BIGINT UNSIGNED NOT NULL,
    supply_note TEXT NOT NULL,
    implementation_note TEXT NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY pl_order_project (project_id),
    UNIQUE KEY pl_order_confirmed_quote (sales_quote_id),
    FOREIGN KEY (project_id) REFERENCES pl_channel_projects(id),
    FOREIGN KEY (project_id, channel_quote_id) REFERENCES pl_channel_quotes(project_id, id),
    FOREIGN KEY (project_id, sales_quote_id) REFERENCES pl_channel_quotes(project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
