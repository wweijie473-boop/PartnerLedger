CREATE TABLE pl_organizations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(150) NOT NULL,
    kind ENUM('MANUFACTURER', 'RESELLER') NOT NULL,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE pl_users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    organization_id BIGINT UNSIGNED NOT NULL,
    username VARCHAR(64) COLLATE utf8mb4_bin NOT NULL,
    password_hash VARBINARY(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY pl_users_username (username),
    CONSTRAINT pl_users_organization_fk FOREIGN KEY (organization_id) REFERENCES pl_organizations (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
