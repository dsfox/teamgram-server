-- A name colour of 0 is red, not "none" (#24).
--
-- The clients number the seven name colours 0..6 and red is 0. Upstream kept
-- "no colour" as 0 as well, so the one colour a person could never choose was
-- red: the server would store it and send nothing. "None" is -1 from here on.
--
-- Every 0 in the table today means "none": nobody could set a colour before
-- account.updateColor was answered, so every row still holds the default.
ALTER TABLE `users`
  MODIFY `color` int NOT NULL DEFAULT '-1',
  MODIFY `profile_color` int NOT NULL DEFAULT '-1';

UPDATE `users` SET `color` = -1 WHERE `color` = 0 AND `color_background_emoji_id` = 0;
UPDATE `users` SET `profile_color` = -1 WHERE `profile_color` = 0 AND `profile_color_background_emoji_id` = 0;
