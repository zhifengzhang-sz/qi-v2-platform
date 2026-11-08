# The state of the test stack, which the tests that reach a stack reach, and which keys them in Go's cache
# (docs/impl/engineering/go.toolchain.md, 4): included by the root Makefile, which names the goals that run those tests
# in STATE_GOALS.
#
# QI_STACK_STATE: the test stack's TimescaleDB, Redpanda and ClickHouse (qitest-*, make stack-test-up), where the
# tests run, each container's name, ID, image's ID, status, health and start, or docker's answer when there is no such
# container. The tests' variables name the test stack's servers, or are unset (QI_KAFKA_BROKERS' test then tries the
# test stack's broker); one naming the stack's servers stops such a goal, FULL=1 too, since the stack holds the
# platform's data and no test runs on it; one naming another server stops it unless FULL=1, which takes nothing from
# the cache and runs against any other.

ifneq ($(filter-out 1,$(FULL)),)
$(error FULL is 1 or unset, not $(FULL))
endif

# which: test, stack or other, for a variable's value ($1) and the test stack's ($2) and the stack's ($3) values
which = $(if $(filter $(2),$(1)),test,$(if $(filter $(3),$(1)),stack,other))

ifneq ($(filter $(STATE_GOALS),$(MAKECMDGOALS)),)
STACKS := $(sort \
  $(if $(QI_WAREHOUSE_TEST_URL),$(call which,$(QI_WAREHOUSE_TEST_URL),postgres://postgres:password@localhost:15432/postgres,postgres://postgres:password@localhost:5432/postgres)) \
  $(if $(QI_KAFKA_BROKERS),$(call which,$(QI_KAFKA_BROKERS),localhost:29092,localhost:19092)) \
  $(if $(QI_CLICKHOUSE_TEST_URL),$(call which,$(QI_CLICKHOUSE_TEST_URL),http://default:password@localhost:18123,http://default:password@localhost:8123)))
ifneq ($(filter stack,$(STACKS)),)
$(error check: QI_WAREHOUSE_TEST_URL, QI_KAFKA_BROKERS or QI_CLICKHOUSE_TEST_URL names the stack's server: the stack holds the platform's data, and tests run on the test stack (make stack-test-up; localhost:15432, 29092, 18123))
endif
ifneq ($(FULL),1)
ifeq ($(shell command -v docker),)
$(error check: no docker: the test stack's state cannot be read; FULL=1 runs every test afresh)
endif
ifneq ($(filter other,$(STACKS)),)
$(error check: QI_WAREHOUSE_TEST_URL, QI_KAFKA_BROKERS or QI_CLICKHOUSE_TEST_URL names a server not the test stack's: FULL=1 runs every test afresh)
endif
FORMAT := {{.Name}} {{.Id}} {{.Image}} {{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}} {{.State.StartedAt}}
export QI_STACK_STATE := $(shell docker inspect -f '$(FORMAT)' qitest-timescaledb qitest-redpanda qitest-clickhouse 2>&1)
endif
endif
