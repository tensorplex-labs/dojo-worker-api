/*
  Warnings:

  - You are about to drop the column `num_criteria` on the `Task` table. All the data in the column will be lost.
  - You are about to drop the column `type` on the `Task` table. All the data in the column will be lost.
  - Added the required column `modality` to the `Task` table without a default value. This is not possible if the table is not empty.

*/
-- CreateEnum
CREATE TYPE "TaskModality" AS ENUM ('CODE_GENERATION', 'IMAGE', 'THREE_D');

-- First, add the new column but allow it to be NULL temporarily
ALTER TABLE "Task" DROP COLUMN "num_criteria",
ADD COLUMN "modality" "TaskModality";

-- Migrate data from old type column to new modality column with appropriate mapping
UPDATE "Task" 
SET "modality" = CASE 
    WHEN "type" = 'CODE_GENERATION' THEN 'CODE_GENERATION'::TaskModality
    WHEN "type" = 'DIALOGUE' THEN 'CODE_GENERATION'::TaskModality -- Default to CODE_GENERATION or decide appropriate mapping
    WHEN "type" = 'TEXT_TO_IMAGE' THEN 'IMAGE'::TaskModality
    WHEN "type" = 'TEXT_TO_THREE_D' THEN 'THREE_D'::TaskModality
    ELSE 'CODE_GENERATION'::TaskModality -- Default fallback
END;

-- Now make the modality column NOT NULL
ALTER TABLE "Task" 
ALTER COLUMN "modality" SET NOT NULL;

-- Finally drop the old type column
ALTER TABLE "Task" 
DROP COLUMN "type";

-- DropEnum
DROP TYPE "TaskType";
