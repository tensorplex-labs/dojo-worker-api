-- Create new enum type
CREATE TYPE "TaskModality" AS ENUM ('CODE_GENERATION', 'IMAGE', 'THREE_D');
CREATE TYPE "TaskType" AS ENUM ('CODE_GENERATION', 'DIALOGUE', 'TEXT_TO_IMAGE', 'TEXT_TO_THREE_D');

-- Delete all tasks with type 'DIALOGUE' as we don't want to migrate them
DELETE FROM "Task" WHERE "type" = 'DIALOGUE'::"TaskType";

-- Add the new column with the new enum type
ALTER TABLE "Task" ADD COLUMN "modality" "TaskModality";

-- Migrate data from the old column to the new column
-- Map 'CODE_GENERATION' to 'CODE_GENERATION'
UPDATE "Task" SET "modality" = 'CODE_GENERATION'::"TaskModality" WHERE "type" = 'CODE_GENERATION'::"TaskType";
-- Map 'TEXT_TO_IMAGE' to 'IMAGE'
UPDATE "Task" SET "modality" = 'IMAGE'::"TaskModality" WHERE "type" = 'TEXT_TO_IMAGE'::"TaskType";
-- Map 'TEXT_TO_THREE_D' to 'THREE_D'
UPDATE "Task" SET "modality" = 'THREE_D'::"TaskModality" WHERE "type" = 'TEXT_TO_THREE_D'::"TaskType";

-- Ensure all tasks have a valid modality value since the schema requires it to be NOT NULL
-- We'll set a default value of CODE_GENERATION for all NULL values
UPDATE "Task" SET "modality" = 'CODE_GENERATION'::"TaskModality" WHERE "modality" IS NULL;

-- Make the modality column NOT NULL
ALTER TABLE "Task" ALTER COLUMN "modality" SET NOT NULL;

-- Drop the old column
ALTER TABLE "Task" DROP COLUMN "type";

-- Drop the old enum type (only after the column using it is gone)
DROP TYPE "TaskType"; 