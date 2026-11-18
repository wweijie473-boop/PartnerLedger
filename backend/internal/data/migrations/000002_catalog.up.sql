CREATE TABLE pl_products (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    manufacturer_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(150) NOT NULL,
    version VARCHAR(100) NOT NULL,
    description VARCHAR(2000) NOT NULL,
    published BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (id),
    INDEX pl_products_manufacturer (manufacturer_id, id),
    INDEX pl_products_published (published, id),
    CONSTRAINT pl_products_manufacturer_fk FOREIGN KEY (manufacturer_id) REFERENCES pl_organizations (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
