ALTER TABLE `channels` 
ADD COLUMN `system_prompt` TEXT DEFAULT NULL COMMENT '渠道级别的系统提示词' AFTER `group`;
