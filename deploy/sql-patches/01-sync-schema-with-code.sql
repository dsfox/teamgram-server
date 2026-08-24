-- Brings the schema in line with what the server code expects.
--
-- The upstream schema lags behind the code: dataobject structs reference columns
-- absent from the SQL. Some of them break things silently (an empty chat member
-- list), others break when the matching feature is used. tests/schema_gate.py
-- finds the mismatches and this file closes them. Types come from the Go struct
-- fields.

-- chat_invite_participants: join requests made through an invite link.
-- A missing `requested` is exactly what made the upstream migration
-- migrate-20260309.sql fail, aborting database initialisation halfway.
ALTER TABLE `chat_invite_participants`
  ADD COLUMN `requested` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `approved_by` bigint(20) NOT NULL DEFAULT '0';

ALTER TABLE `chat_invite_participants`
  ADD KEY `idx_chat_requested` (`chat_id`, `requested`);

-- dialog_filters: marks a folder created from a suggested one.
ALTER TABLE `dialog_filters`
  ADD COLUMN `from_suggested` int(11) NOT NULL DEFAULT '0';

-- documents: soft deletion and the link to an imported document.
ALTER TABLE `documents`
  ADD COLUMN `import_document_id` bigint(20) NOT NULL DEFAULT '0',
  ADD COLUMN `deleted` tinyint(1) NOT NULL DEFAULT '0';

-- photo_sizes: file coordinates in the old storage scheme and the stripped thumbnail.
ALTER TABLE `photo_sizes`
  ADD COLUMN `volume_id` bigint(20) NOT NULL DEFAULT '0',
  ADD COLUMN `local_id` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `secret` bigint(20) NOT NULL DEFAULT '0',
  ADD COLUMN `has_stripped` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `stripped_bytes` varchar(255) NOT NULL DEFAULT '';

-- bots: mini app fields and the attachment menu.
ALTER TABLE `bots`
  ADD COLUMN `bot_attach_menu` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `attach_menu_enabled` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `bot_business` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `bot_has_main_app` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `bot_active_users` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `has_menu_button` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `menu_button_text` varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN `menu_button_url` varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN `bot_can_edit` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `has_preview_medias` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `description_photo_id` bigint(20) NOT NULL DEFAULT '0',
  ADD COLUMN `description_document_id` bigint(20) NOT NULL DEFAULT '0',
  ADD COLUMN `main_app_url` varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN `has_app_settings` tinyint(1) NOT NULL DEFAULT '0',
  ADD COLUMN `placeholder_path` varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN `background_color` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `background_dark_color` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `header_color` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `header_dark_color` int(11) NOT NULL DEFAULT '0',
  ADD COLUMN `privacy_policy_url` varchar(255) NOT NULL DEFAULT '',
  ADD COLUMN `mode` int(11) NOT NULL DEFAULT '0';

-- messages: when the recipient read an outgoing message.
ALTER TABLE `messages`
  ADD COLUMN `outbox_read_date` bigint(20) NOT NULL DEFAULT '0';
