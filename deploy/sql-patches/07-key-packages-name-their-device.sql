-- Which identity a key package belongs to.
--
-- A device that starts its state over - after a reinstall, or because the state
-- it had was named for somebody else - makes a new MLS identity. The packages it
-- published under the old one stay on the server, and the server counts what a
-- device has by its auth key rather than by its identity: it sees a full supply,
-- says "no need to publish", and the new identity never publishes anything.
--
-- Everybody who then starts a conversation with that person claims a package of
-- the identity that is gone, and builds an invitation the person can never open.
-- It happened to a real account: fifty-four packages from 22 August, none since,
-- and every welcome silently unopenable (#136).
--
-- Empty for what is already there, which is what makes the cure automatic: a
-- package whose name does not match the one the device says it has now is
-- thrown away on that device's next publish, and the supply refills.
ALTER TABLE `mls_key_packages`
  ADD COLUMN `name` varbinary(255) NOT NULL DEFAULT '' AFTER `fingerprint`;
