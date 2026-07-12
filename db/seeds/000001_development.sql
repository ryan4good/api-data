-- Deterministic local-development data. Never apply this seed in production.
SET NAMES utf8mb4;
SET time_zone = '+00:00';

INSERT INTO users (id, email, display_name, platform_role, status) VALUES
  ('10000000-0000-4000-8000-000000000001', 'alice@example.test', 'Alice OMS Owner', 'member', 'active'),
  ('10000000-0000-4000-8000-000000000002', 'bob@example.test', 'Bob WMS Maintainer', 'member', 'active'),
  ('10000000-0000-4000-8000-000000000003', 'viewer@example.test', 'OMS Viewer', 'member', 'active'),
  ('10000000-0000-4000-8000-000000000004', 'outsider@example.test', 'No System Access', 'member', 'active')
ON DUPLICATE KEY UPDATE
  display_name = VALUES(display_name),
  platform_role = VALUES(platform_role),
  status = VALUES(status);

INSERT INTO business_systems (id, system_key, name, description, status, created_by) VALUES
  ('20000000-0000-4000-8000-000000000001', 'oms', 'Order Management System', 'Local OMS fixture', 'active', '10000000-0000-4000-8000-000000000001'),
  ('20000000-0000-4000-8000-000000000002', 'wms', 'Warehouse Management System', 'Local WMS fixture', 'active', '10000000-0000-4000-8000-000000000002')
ON DUPLICATE KEY UPDATE
  name = VALUES(name),
  description = VALUES(description),
  status = VALUES(status);

INSERT INTO system_members (id, system_id, user_id, role) VALUES
  ('30000000-0000-4000-8000-000000000001', '20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000001', 'owner'),
  ('30000000-0000-4000-8000-000000000002', '20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000002', 'maintainer'),
  ('30000000-0000-4000-8000-000000000003', '20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000003', 'viewer')
ON DUPLICATE KEY UPDATE role = VALUES(role);
