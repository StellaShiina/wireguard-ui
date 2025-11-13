-- Initialize PostgreSQL extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- server table: only one row, uuid primary key, insert fixed uuid at initialization, deletion not allowed, only updates allowed
CREATE TABLE IF NOT EXISTS server (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    public_ip INET NOT NULL,
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    enable_ipv6 BOOLEAN NOT NULL DEFAULT TRUE,
    subnet_v4 CIDR NOT NULL,
    subnet_v6 CIDR NOT NULL,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL
);

-- Prevent multiple insertions & prohibit deletion of server row
CREATE OR REPLACE FUNCTION enforce_singleton_server()
RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF (SELECT COUNT(*) FROM server) >= 1 THEN
            RAISE EXCEPTION 'Server table allows only one row';
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'Deletion from server is not allowed';
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS server_insert_guard ON server;
CREATE TRIGGER server_insert_guard
BEFORE INSERT ON server
FOR EACH ROW EXECUTE FUNCTION enforce_singleton_server();

DROP TRIGGER IF EXISTS server_delete_guard ON server;
CREATE TRIGGER server_delete_guard
BEFORE DELETE ON server
FOR EACH ROW EXECUTE FUNCTION enforce_singleton_server();

-- peer table: uuid primary key, IPv4(/32), IPv6(/128) automatically assigned; only name field can be updated; entire row can be deleted
CREATE TABLE IF NOT EXISTS peer (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ipv4 CIDR UNIQUE,
    ipv6 CIDR UNIQUE,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    name TEXT
);

-- Calculate the next available IPv4 (/32)
CREATE OR REPLACE FUNCTION get_next_free_ipv4()
RETURNS CIDR AS $$
DECLARE
    subnet_v4 CIDR;
    start_ip INET;
    end_ip INET;
    current_ip INET;
    used_ip CIDR;
BEGIN
    SELECT s.subnet_v4 INTO subnet_v4 FROM server s LIMIT 1;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Server subnet_v4 not configured';
    END IF;

    -- Allocate from the first host address (.1), reserving network address .0
    start_ip := set_masklen(subnet_v4 + 1, 32);
    end_ip := set_masklen(broadcast(subnet_v4) - 1, 32);

    IF start_ip > end_ip THEN
        RAISE EXCEPTION 'Invalid IPv4 range: start_ip (%) > end_ip (%)', start_ip, end_ip;
    END IF;

    current_ip := start_ip;
    WHILE current_ip <= end_ip LOOP
        SELECT p.ipv4 INTO used_ip FROM peer p WHERE p.ipv4 = set_masklen(current_ip, 32);
        IF NOT FOUND THEN
            RETURN set_masklen(current_ip, 32);
        END IF;
        current_ip := current_ip + 1;
    END LOOP;

    RAISE EXCEPTION 'No free IPv4 addresses available in subnet %', subnet_v4;
END;
$$ LANGUAGE plpgsql;

-- Calculate the next available IPv6 (/128)
CREATE OR REPLACE FUNCTION get_next_free_ipv6()
RETURNS CIDR AS $$
DECLARE
    subnet_v6 CIDR;
    current_ip INET;
    used_ip CIDR;
    attempts INTEGER := 0;
BEGIN
    SELECT s.subnet_v6 INTO subnet_v6 FROM server s LIMIT 1;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Server subnet_v6 not configured';
    END IF;

    -- Allocate from the first address of the subnet + 1 (reserving the first address)
    current_ip := set_masklen(subnet_v6, 128) + 1;

    LOOP
        attempts := attempts + 1;
        IF attempts > 100000 THEN
            RAISE EXCEPTION 'Exceeded allocation attempts for IPv6 in subnet %', subnet_v6;
        END IF;

        SELECT p.ipv6 INTO used_ip FROM peer p WHERE p.ipv6 = set_masklen(current_ip, 128);
        IF NOT FOUND THEN
            RETURN set_masklen(current_ip, 128);
        END IF;
        current_ip := current_ip + 1;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Before insert trigger: automatically assign IPv4/IPv6
CREATE OR REPLACE FUNCTION peer_before_insert()
RETURNS trigger AS $$
BEGIN
    IF NEW.ipv4 IS NULL THEN
        NEW.ipv4 := get_next_free_ipv4();
    END IF;
    IF NEW.ipv6 IS NULL THEN
        NEW.ipv6 := get_next_free_ipv6();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS peer_auto_assign ON peer;
CREATE TRIGGER peer_auto_assign
BEFORE INSERT ON peer
FOR EACH ROW EXECUTE FUNCTION peer_before_insert();

-- Before update trigger: only allow updating the name field
CREATE OR REPLACE FUNCTION peer_before_update_only_name()
RETURNS trigger AS $$
BEGIN
    IF NEW.uuid <> OLD.uuid OR
       NEW.ipv4 <> OLD.ipv4 OR
       NEW.ipv6 <> OLD.ipv6 OR
       NEW.private_key <> OLD.private_key OR
       NEW.public_key <> OLD.public_key THEN
        RAISE EXCEPTION 'Only the name field can be updated for peer records';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS peer_update_guard ON peer;
CREATE TRIGGER peer_update_guard
BEFORE UPDATE ON peer
FOR EACH ROW EXECUTE FUNCTION peer_before_update_only_name();

-- Initialize server row with fixed uuid (skip if already exists)
INSERT INTO server (uuid, public_ip, port, enable_ipv6, subnet_v4, subnet_v6, private_key, public_key)
SELECT '00000000-0000-0000-0000-000000000001', '203.0.113.1', 51820, TRUE, '10.7.21.0/24', 'fd00:7:21::/64', 'SERVER_PRIVATE_KEY', 'SERVER_PUBLIC_KEY'
WHERE NOT EXISTS (SELECT 1 FROM server);