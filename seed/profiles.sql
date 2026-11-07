-- Seed data, loaded only into an empty database after AutoMigrate.
-- Column names follow GORM's naming of internal/model structs.
-- Partition sizes are sectors; size 0 means "to the end of the disk".
-- Types: DOS uses hex (82 swap, 83 linux); GPT uses sfdisk aliases
-- (U = EFI system, S = swap, L = linux filesystem).

INSERT INTO layouts (id, name) VALUES
  (1, 'vm-bios-dos'),
  (2, 'vm-efi-gpt'),
  (3, 'nvme-efi-gpt');

INSERT INTO templates (id, name, label, sector_size) VALUES
  (1, 'dos-swap-root', 'dos', 512),
  (2, 'gpt-efi-swap-root', 'gpt', 512);

INSERT INTO layout_templates (id, layout_id, template_id, map_key) VALUES
  (1, 1, 1, 'vda'),
  (2, 2, 2, 'vda'),
  (3, 3, 2, 'nvme0n1');

INSERT INTO partitions (id, template_id, start, size, type, name,
  filesystem, mount_point, mount_options, is_bootable) VALUES
  (1, 1, 2048, 4194304, '82', '', 'swap', '', '', 0),
  (2, 1, 4196352, 0, '83', '', 'ext4', '/', 'defaults', 1),
  (3, 2, 2048, 1048576, 'U', 'efi', 'vfat', '/boot/efi', 'defaults', 0),
  (4, 2, 1050624, 4194304, 'S', 'swap', 'swap', '', '', 0),
  (5, 2, 5244928, 0, 'L', 'root', 'ext4', '/', 'defaults', 0);

INSERT INTO profiles (id, layout_id, hardware_address, ip_address, fqdn,
  distro, repo, packages, authorized_keys, root_password_hash,
  is_build_enabled) VALUES
  (1, 1, '52:54:00:76:de:e5', '172.168.16.32',
   'glarg01.dreamtrack.net', 'void', '', '', '', '', 1),
  (2, 3, '94:c6:91:a0:36:98', '172.168.16.214',
   'vlissides.dreamtrack.net', 'void', '', '', '', '', 0);
