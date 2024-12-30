-- CreateEnum
CREATE TYPE "TaskStatus" AS ENUM ('IN_PROGRESS', 'COMPLETED', 'EXPIRED');

-- CreateEnum
CREATE TYPE "TaskResultStatus" AS ENUM ('IN_PROGRESS', 'COMPLETED', 'INVALID');

-- CreateEnum
CREATE TYPE "TaskType" AS ENUM ('CODE_GENERATION', 'DIALOGUE', 'TEXT_TO_IMAGE', 'TEXT_TO_THREE_D');

-- CreateEnum
CREATE TYPE "MetricsType" AS ENUM ('TOTAL_NUM_DOJO_WORKERS', 'TOTAL_NUM_COMPLETED_TASKS', 'TOTAL_NUM_TASK_RESULTS', 'AVERAGE_TASK_COMPLETION_TIME');

-- CreateEnum
CREATE TYPE "EventsType" AS ENUM ('TASK_COMPLETION_TIME');

-- CreateTable
CREATE TABLE "ExternalUser" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "name" TEXT NOT NULL,
    "email" TEXT NOT NULL,
    "is_verified" BOOLEAN NOT NULL DEFAULT false,

    CONSTRAINT "ExternalUser_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "ApiKey" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "key" TEXT NOT NULL,
    "is_delete" BOOLEAN NOT NULL DEFAULT false,
    "miner_user_id" TEXT NOT NULL,

    CONSTRAINT "ApiKey_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "SubscriptionKey" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "key" TEXT NOT NULL,
    "is_delete" BOOLEAN NOT NULL DEFAULT false,
    "miner_user_id" TEXT NOT NULL,

    CONSTRAINT "SubscriptionKey_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "MinerUser" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "hotkey" TEXT NOT NULL,
    "email" TEXT,
    "organizationName" TEXT,

    CONSTRAINT "MinerUser_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Task" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "expire_at" TIMESTAMP(3) NOT NULL,
    "title" TEXT NOT NULL,
    "body" TEXT NOT NULL,
    "type" "TaskType" NOT NULL,
    "task_data" JSONB NOT NULL,
    "status" "TaskStatus" NOT NULL,
    "max_results" INTEGER NOT NULL,
    "num_results" INTEGER NOT NULL,
    "num_criteria" INTEGER NOT NULL,
    "total_reward" DOUBLE PRECISION,
    "miner_user_id" TEXT,

    CONSTRAINT "Task_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "TaskResult" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "status" "TaskResultStatus" NOT NULL,
    "result_data" JSONB NOT NULL,
    "task_id" TEXT NOT NULL,
    "worker_id" TEXT NOT NULL,
    "stake_amount" DOUBLE PRECISION,
    "potential_reward" DOUBLE PRECISION,
    "potential_loss" DOUBLE PRECISION,
    "finalised_reward" DOUBLE PRECISION,
    "finalised_loss" DOUBLE PRECISION,

    CONSTRAINT "TaskResult_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "DojoWorker" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "wallet_address" TEXT NOT NULL,
    "chain_id" TEXT NOT NULL,
    "current_stake_amount" DOUBLE PRECISION,

    CONSTRAINT "DojoWorker_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "WorkerPartner" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "miner_subscription_key" TEXT NOT NULL,
    "worker_id" TEXT NOT NULL,
    "is_delete_by_miner" BOOLEAN NOT NULL DEFAULT false,
    "is_delete_by_worker" BOOLEAN NOT NULL DEFAULT false,
    "name" TEXT,

    CONSTRAINT "WorkerPartner_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Metrics" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "type" "MetricsType" NOT NULL,
    "metrics_data" JSONB NOT NULL,

    CONSTRAINT "Metrics_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Events" (
    "id" TEXT NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "type" "EventsType" NOT NULL,
    "events_data" JSONB NOT NULL,

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
ALTER TABLE "ApiKey" ADD CONSTRAINT "ApiKey_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "SubscriptionKey" ADD CONSTRAINT "SubscriptionKey_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Task" ADD CONSTRAINT "Task_miner_user_id_fkey" FOREIGN KEY ("miner_user_id") REFERENCES "MinerUser"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "TaskResult" ADD CONSTRAINT "TaskResult_task_id_fkey" FOREIGN KEY ("task_id") REFERENCES "Task"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "TaskResult" ADD CONSTRAINT "TaskResult_worker_id_fkey" FOREIGN KEY ("worker_id") REFERENCES "DojoWorker"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "WorkerPartner" ADD CONSTRAINT "WorkerPartner_miner_subscription_key_fkey" FOREIGN KEY ("miner_subscription_key") REFERENCES "SubscriptionKey"("key") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "WorkerPartner" ADD CONSTRAINT "WorkerPartner_worker_id_fkey" FOREIGN KEY ("worker_id") REFERENCES "DojoWorker"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
