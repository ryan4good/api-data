-- Application query contract.
-- The application must bind the authenticated user ID; never accept user_id
-- from an untrusted request payload.

-- name: ListAuthorizedBusinessSystems
SELECT bs.id, bs.system_key, bs.name, bs.description, bs.status,
       sm.role, bs.created_at, bs.updated_at
FROM business_systems AS bs
JOIN system_members AS sm
  ON sm.system_id = bs.id
 AND sm.user_id = ?
WHERE bs.status = 'active'
ORDER BY bs.name, bs.id;

-- name: GetAuthorizedBusinessSystem
SELECT bs.id, bs.system_key, bs.name, bs.description, bs.status,
       sm.role, bs.created_at, bs.updated_at
FROM business_systems AS bs
JOIN system_members AS sm
  ON sm.system_id = bs.id
 AND sm.user_id = ?
WHERE bs.id = ?
  AND bs.status = 'active';
