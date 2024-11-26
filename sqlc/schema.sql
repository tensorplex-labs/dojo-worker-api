-- CreateEnum
CREATE TYPE "TaskStatus" AS ENUM(
    'IN_PROGRESS',
    'COMPLETED',
    'EXPIRED'
);

-- CreateEnum
CREATE TYPE "TaskResultStatus" AS ENUM(
    'IN_PROGRESS',
    'COMPLETED',
    'INVALID'
);

-- CreateEnum
CREATE TYPE "TaskType" AS ENUM(
    'CODE_GENERATION',
    'DIALOGUE',
    'TEXT_TO_IMAGE',
    'TEXT_TO_THREE_D'
);

-- CreateEnum
CREATE TYPE "MetricsType" AS ENUM(
    'TOTAL_NUM_DOJO_WORKERS',
    'TOTAL_NUM_COMPLETED_TASKS',
    'TOTAL_NUM_TASK_RESULTS',
    'AVERAGE_TASK_COMPLETION_TIME'
);

-- CreateEnum
CREATE TYPE "EventsType" AS ENUM(
    'TASK_COMPLETION_TIME'
);

-- CreateTable
CREATE TABLE "ExternalUser"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "name" text NOT NULL,
    "email" text NOT NULL,
    "is_verified" boolean NOT NULL DEFAULT FALSE,
    CONSTRAINT "ExternalUser_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "ApiKey"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "key" text NOT NULL,
    "is_delete" boolean NOT NULL DEFAULT FALSE,
    "miner_user_id" text NOT NULL,
    CONSTRAINT "ApiKey_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "SubscriptionKey"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "key" text NOT NULL,
    "is_delete" boolean NOT NULL DEFAULT FALSE,
    "miner_user_id" text NOT NULL,
    CONSTRAINT "SubscriptionKey_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "MinerUser"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "hotkey" text NOT NULL,
    "email" text,
    "organizationName" text,
    CONSTRAINT "MinerUser_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Task"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "expire_at" timestamp(3) NOT NULL,
    "title" text NOT NULL,
    "body" text NOT NULL,
    "type" "TaskType" NOT NULL,
    "task_data" jsonb NOT NULL,
    "status" "TaskStatus" NOT NULL,
    "max_results" integer NOT NULL,
    "num_results" integer NOT NULL,
    "num_criteria" integer NOT NULL,
    "total_reward" double precision,
    "miner_user_id" text,
    CONSTRAINT "Task_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "TaskResult"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "status" "TaskResultStatus" NOT NULL,
    "result_data" jsonb NOT NULL,
    "task_id" text NOT NULL,
    "worker_id" text NOT NULL,
    "stake_amount" double precision,
    "potential_reward" double precision,
    "potential_loss" double precision,
    "finalised_reward" double precision,
    "finalised_loss" double precision,
    CONSTRAINT "TaskResult_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "DojoWorker"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "wallet_address" text NOT NULL,
    "chain_id" text NOT NULL,
    "current_stake_amount" double precision,
    CONSTRAINT "DojoWorker_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "WorkerPartner"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "miner_subscription_key" text NOT NULL,
    "worker_id" text NOT NULL,
    "is_delete_by_miner" boolean NOT NULL DEFAULT FALSE,
    "is_delete_by_worker" boolean NOT NULL DEFAULT FALSE,
    "name" text,
    CONSTRAINT "WorkerPartner_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Metrics"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "type" "MetricsType" NOT NULL,
    "metrics_data" jsonb NOT NULL,
    CONSTRAINT "Metrics_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Events"(
    "id" text NOT NULL,
    "created_at" timestamp(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamp(3) NOT NULL,
    "type" "EventsType" NOT NULL,
    "events_data" jsonb NOT NULL,
    CONSTRAINT "Events_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "ApiKey_key_key" ON "ApiKey"("key");

-- CreateIndex
CREATE UNIQUE INDEX "SubscriptionKey_key_key" ON "SubscriptionKey"("key");

-- CreateIndex
CREATE UNIQUE INDEX "MinerUser_hotkey_key" ON "MinerUser"("hotkey");

-- CreateIndex
CREATE UNIQUE INDEX "DojoWorker_wallet_address_chain_id_key" ON "DojoWorker"("wallet_address", "chain_id");

-- CreateIndex
CREATE UNIQUE INDEX "WorkerPartner_miner_subscription_key_worker_id_key" ON "WorkerPartner"("miner_subscription_key", "worker_id");

-- CreateIndex
CREATE UNIQUE INDEX "Metrics_type_key" ON "Metrics"("type");

-- AddForeignKey
ALTER TABLE "ApiKey"
    ADD CONSTRAINT "ApiKey_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "SubscriptionKey"
    ADD CONSTRAINT "SubscriptionKey_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Task"
    ADD CONSTRAINT "Task_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "TaskResult"
    ADD CONSTRAINT "TaskResult_task_id_fkey" FOREIGN KEY ("task_id") REFERENCES "Task"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "TaskResult"
    ADD CONSTRAINT "TaskResult_worker_id_fkey" FOREIGN KEY ("worker_id") REFERENCES "DojoWorker"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "WorkerPartner"
    ADD CONSTRAINT "WorkerPartner_miner_subscription_key_fkey" FOREIGN KEY ("miner_subscription_key") REFERENCES "SubscriptionKey"("key") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "WorkerPartner"
    ADD CONSTRAINT "WorkerPartner_worker_id_fkey" FOREIGN KEY ("worker_id") REFERENCES "DojoWorker"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

