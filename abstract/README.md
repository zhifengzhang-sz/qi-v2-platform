# abstract-level: the definition's interfaces in Go

`abstract/`, a Go module of its own (`github.com/zhifengzhang-sz/qi-v2-platform/abstract`, package `abstract`), on the
standard library alone. It is the platform's formal definition (`qi-v2-platform-definition` 0.1.0, in Lean, the
flake's input `definition`, at the shell's `QI_DEFINITION`) as far as code can hold it: the open parts of the working
space's components are Go interfaces, and what the definition fixes is built in. Every other product of the working
space is built on it: a component implements its parts, a binding gives the logs.

## What it holds

**A log and its two ends.** `Log[T]` holds one log, a sequence of messages that only grows; it is given from outside
this package, Kafka's by a binding, memory's in a test. A part never holds a log: it is given a `Cursor[T]`, a reading
end (the log's messages in order from the first, each once, `Next` waiting for the next), or an `Appender[T]`, the
writing end (`Append`, and nothing else).

**A topic's shape.** `Topic[K, V]` is a topic's description: its name, its number of partitions, at least one
(`NewTopic` refuses fewer), and the types of its keys and values. `Record[K, V]` is a record as Kafka gives it to a
reader. `TopicLogs` is a topic with what it holds, one log of records for each partition (`Topic.Logs`, which refuses
any other number of logs, and a log given twice where that can be told, `ErrSameLog`). `AnyTopic` is a topic of any schema. A part is given topics as a set: a `ReadSet`, from
which `ReadFrom` gives a cursor on one partition, or an `AppendSet`, from which `AppendTo` gives one partition's
writing end; a topic or a partition the set does not hold is refused (`ErrNotGiven`). The two sets are two types,
and no conversion makes one of the other.

**The parts, the open ones.** Each is an interface of one method, and a call of the method is the part at work, on
the ends it is given:

| Part | Interface | Its method is given |
|---|---|---|
| acquisition's reader | `Reader[Src, Kline]` | the source to read, the raw series to append to |
| acquisition's cleaner | `Cleaner[Kline]` | the raw series to read, the clean series to append to |
| acquisition's publisher, one part a kind | `RawPublisher[Kline]`, `CleanPublisher[Kline]`, `CorrectionsPublisher[Corr]` | its series to read (the raw series, the clean series, the corrections stream), the topics of its kind to append to |
| the warehouse consumer, one a schema | `Consumer[K, V, P]` | one partition of a topic of its schema to read, its pushes to append to |
| visualization's chart | `Chart[View]` | its clean topics and the engine's topics to read, the chart's updates to append to |
| the engine's worker | `Worker` | its clean topics to read, the engine's own topics to append to |

**What is built in.** The three kinds of data acquisition publishes (`Kind`: `Raw`, `Clean`, `Corrections`); and the
wiring of a working space (`Wiring`, and the `Workspace` it makes):

- acquisition alone reads the source and the corrections (`Inputs`): `Workspace.Run` gives them to `Acquisition`, and
  to nothing else; they are two logs, and one cursor given as both is refused (`ErrSameInput`), where it is given as
  one pointer or one channel;
- the clean series is made from the raw: the cleaner is given the reader's series, and the clean publisher the
  cleaner's;
- the corrections are applied to nothing: the corrections stream reaches its publisher alone;
- each publisher writes the topics of its kind alone: the set it is given holds no other;
- warehousing reads every topic of acquisition's, a partition at a time, into the topic's table: the consumer of the
  topic's schema is called once for each partition, with that partition's cursor, and what it pushes goes to the
  `Database`'s log for the topic's table (`Wiring.Tables`) and that partition;
- the engine is optional (`Wiring.Engine`, nil for none) and given clean topics only: each topic it names is one of
  acquisition's clean topics, or the working space is refused (`ErrNotClean`);
- visualization is given its clean topics, held to the same, and all of the engine's, which are none without it;
- two topics of one name, of acquisition's or the engine's or one of each, are refused when the working space is
  made (`ErrSameName`); so is one log given for two partitions, of one topic or of two (`ErrSameLog`), where the two
  are given as one pointer or one channel; so is a part that is missing, a topic of acquisition's with no consumer of its schema or no
  table, and a table for a topic acquisition has not;
- a part can only append: every writing end a part is given is an `Appender` and nothing more, every reading end a
  `Cursor` and nothing more;
- a working space is run once, a second `Run` refused, of it or of a copy of it: within one, nothing starts again.
  A `Workspace` that no wiring made is refused.

`Acquisition.Run` and `Warehousing.Run` are the two components each as one process, which `Workspace.Run` runs with
the engine's worker and the chart. Alone, each finds a topic by its name, and refuses two of one name among the
topics of a kind, or among those it reads, and one log given for two partitions. Inside acquisition the raw series and the clean series hold, for each part
that reads them, what it has not taken yet: no part waits for another to read, so a part that reads nothing holds no
other back. A run returns once every part has returned, so that nothing of it is left running: with nil where each
returned nil, else with the first error a part returned, which is the context's where the run ended because its
context was done.

## What the types do not hold

The definition holds each part to Kahn's condition: more input never takes back output, and no output waits for the
end of the input. The types give a part no means to take back what it appended, and no end to wait for. The rest is
the implementation's to keep, and nothing here checks it:

- what a part appends depends on what it has read and on nothing else: not on the time, and not on which of two
  cursors has a message first. A part that reads several cursors, the worker and the chart, waits on one at a time,
  in an order that what it has read decides; reading them as their messages arrive is a merge by arrival, which is no
  process;
- a part does not treat its context's end as an end of its input: it returns, and appends nothing for it;
- what a part keeps between messages it keeps in the call: `Consumer.Consume` is called once for each partition, and
  each call is the consumer on that partition alone;
- a `Log` a binding gives is the whole log, in one order for every cursor, and not what retention or compaction leave
  of it; two logs given are two logs, which is told only where one pointer or one channel is given twice; what is given from
  outside the topics (the inputs, the chart's log, a database's logs) is no topic's log; and the two inputs are two
  logs, which is told in the same way;
- a `Database` holds a table as the set of the rows pushed into it, a row a message marked with the log it is from;
  and the `Database` values given with the consumers of several schemas, each of its own type of push, are one
  database, in which a table's name names one table.

Three places hold less in the types than the definition does, and check it when the working space is made instead:
a clean topic is one by its type in Lean, and here by the check that refuses any other; the names of the topics are
distinct by a proof in Lean, and here by the check that refuses a name twice; the warehouse consumer is given for every
schema in Lean, and here for the schemas of acquisition's topics, a topic with none refused.

## The files

| File | Holds |
|---|---|
| `log.go` | `Log`, `Appender`, `Cursor` |
| `topic.go` | `Record`, `Header`, `Topic`, `AnyTopic`, `TopicLogs`, `ReadSet`, `AppendSet`, the five refusals |
| `acquisition.go` | `Kind`, `AllTopics`, `Inputs`, the reader, the cleaner, the three publishers, `Acquisition` and its `Run` |
| `warehousing.go` | `Consumer`, `Database`, `SchemaConsumer`, `Warehousing` and its `Run` |
| `visualization.go` | `Chart` |
| `engine.go` | `Worker`, `Engine` |
| `workspace.go` | `Component`, `Wiring`, `Workspace` and its `Run` |
| `run.go` | how a run's parts are run and stopped together; the series inside acquisition, which hold for each reader what it has not taken |

## Taking it

The flake's package `abstract-level`, on `x86_64-linux`: this module as source, without its tests, with this file
and the repository's `LICENSE`. In the package the module's files are at its root, and nothing else of the
repository is there: the tests, the paths and the targets this file names (`make check-abstract`, `QI_DEFINITION`,
the tests of the table below) are the repository's. Its release is 0.1.0, the tag `abstract-level-0.1.0`; a release
is a tag `abstract-level-<version>` (Changes, below), read over HTTPS with no login. A taker pins the repository at
the tag, an input of its `flake.nix` (`github:zhifengzhang-sz/qi-v2-platform?ref=abstract-level-0.1.0`), and its
`go.mod` requires the module at `v0.0.0` and replaces its path with the package's store path:

    require github.com/zhifengzhang-sz/qi-v2-platform/abstract v0.0.0

    replace github.com/zhifengzhang-sz/qi-v2-platform/abstract v0.0.0 => /nix/store/…-abstract-level-0.1.0

The module takes the standard library alone, so its `go.sum` needs no line for it.

## Its check

`make check-abstract`, from the root, in the shell: `gofmt` with no unformatted file, `go vet`, `go test`, with
`GOPROXY=off`; it is in `make check`, and `make verify` runs it for a change under `abstract/` or of the flake.
`make check-abstract-package` checks the package as a taker takes it: built offline, then taken by a module in a
temporary folder whose `go.mod` replaces the module's path with the package's store path, `gofmt`, `go vet` and
`go test` of its test (`dev/taker/abstract-level/`), which wires a working space from what the package exports and
runs it, with `GOPROXY=off`; `make verify` runs it for a change under `abstract/`, of the flake, of `LICENSE` or of
that test. The module's tests:

- run a working space on topics, a database and a chart in memory, on finite inputs, and hold what each of the
  wiring's theorems states, the theorem's two sides compared: what the run wrote, and what the parts themselves
  give, called one on the other's output as the theorem composes them (`wiring_test.go`; the table below names each
  theorem's test);
- hold what is built in beside them: a topic's shape, what each part is given and that it is the one end, each
  refusal, a run made once, a failing part and a context that is done each stopping the whole, a part that reads
  nothing holding no other back (`space_test.go`);
- read the pinned definition at `QI_DEFINITION` and this file's table (`TestDefinition`): a declaration of the seven
  files with no row fails it, and so does a row for a declaration its file does not make, a row that names Go this
  package does not have, and a row that names no Go and gives no reason.

## The definition, declaration by declaration

Each declaration written in the definition's `Kafka.lean`, `Acquisition.lean`, `Warehousing.lean`,
`Visualization.lean`, `MarketModelEngine.lean`, `Workspace.lean` and `Wiring.lean`, by its full name, a structure's
fields as `Structure.field` and an inductive's constructors as `Inductive.constructor`, with its Go name here, or why
it is not in code. A Go name is a name of this package: `Type.Name` is a method, a field or an interface's method,
`Type[Name]` a type's parameter, `Test…` the test that holds what a theorem states.

Of the definition's base (`Kahn`), which those files are written on: a log (`Kahn.Log`) is a `Log`, seen by a part as
a `Cursor` or an `Appender`; a process (`Kahn.Process`) is a part, a call of its method; a set of rows (`Kahn.Rows`)
is a table of a `Database`.

### `Kafka.lean`

| Declaration | Go | Note |
|---|---|---|
| `Kafka.Record` | `Record` | |
| `Kafka.Record.key` | `Record.Key` | absent is nil |
| `Kafka.Record.value` | `Record.Value` | absent is nil |
| `Kafka.Record.timestamp` | `Record.Timestamp` | an `int64`, Kafka's, where Lean's `Int` has no bound |
| `Kafka.Record.headers` | `Record.Headers`, `Header` | a header's value absent is nil |
| `Kafka.Topic` | `Topic`, `AnyTopic`, `NewTopic` | |
| `Kafka.Topic.name` | `Topic.Name` | |
| `Kafka.Topic.partitions` | `Topic.Partitions` | |
| `Kafka.Topic.partitions_pos` | `NewTopic`, `Topic.Logs`, `TestTopic` | fewer than one refused when the topic is made; the zero `Topic`, of none, refused when it is given logs |
| `Kafka.Topic.Key` | `Topic[K]`, `Topic.Key` | |
| `Kafka.Topic.Value` | `Topic[V]`, `Topic.Value` | |
| `Kafka.Topic.Logs` | `TopicLogs`, `Topic.Logs`, `Log`, `ErrSameLog` | one log a partition, no more and no fewer, and none given twice where that can be told |
| `Kafka.compaction_not_process` | | not in code: a theorem that no process gives a view that replaces a record. A `Log` has no means to replace or remove one; that a binding's log is the whole log is the binding's to keep |

### `Acquisition.lean`

| Declaration | Go | Note |
|---|---|---|
| `Acquisition.Kind` | `Kind` | |
| `Acquisition.Kind.raw` | `Raw` | |
| `Acquisition.Kind.clean` | `Clean` | |
| `Acquisition.Kind.corrections` | `Corrections` | |
| `Acquisition.Data` | `Acquisition`, `AllTopics` | no type of its own: its three types are `Acquisition`'s parameters, its topics `AllTopics` |
| `Acquisition.Data.Kline` | `Acquisition[Kline]` | |
| `Acquisition.Data.Source` | `Acquisition[Src]` | |
| `Acquisition.Data.Correction` | `Acquisition[Corr]` | |
| `Acquisition.Data.Topic` | `AllTopics`, `AllTopics.Of` | the topics of a kind are a list, where Lean has a type |
| `Acquisition.Data.topic` | `TopicLogs.Topic` | each entry of `AllTopics` is a topic with its logs |
| `Acquisition.Series` | `series` | a log of k-lines inside acquisition, held for each part that reads it from what that part has not taken; its ends are a `Cursor[Kline]` and an `Appender[Kline]` |
| `Acquisition.Topics` | `AllTopics.Of`, `AppendSet` | the topics of a kind, which that kind's publisher is given to append to |
| `Acquisition.AllTopics` | `AllTopics` | |
| `Acquisition.In` | `Inputs` | |
| `Acquisition` | `Acquisition` | |
| `Acquisition.reader` | `Reader`, `Acquisition.Reader` | |
| `Acquisition.cleaner` | `Cleaner`, `Acquisition.Cleaner` | |
| `Acquisition.publishRaw` | `RawPublisher`, `Acquisition.PublishRaw` | |
| `Acquisition.publishClean` | `CleanPublisher`, `Acquisition.PublishClean` | |
| `Acquisition.publishCorrections` | `CorrectionsPublisher`, `Acquisition.PublishCorrections` | |
| `Acquisition.run` | `Acquisition.Run` | a publisher finds a topic by its name: two topics of a kind of one name are refused, and so is one log given for two partitions, where Lean's `run` takes any topics |
| `Acquisition.raw_topics` | `TestRawTopics` | |
| `Acquisition.clean_topics` | `TestCleanTopics` | |
| `Acquisition.corrections_topics` | `TestCorrectionsTopics` | |

### `Warehousing.lean`

| Declaration | Go | Note |
|---|---|---|
| `Warehousing.Data` | `Warehousing.Run`, `Consumer` | no type of its own: its topics are those `Warehousing.Run` is given, its `Push` the consumers' `P` |
| `Warehousing.Data.Topic` | `Warehousing.Run` | the topics it is given, a list |
| `Warehousing.Data.topic` | `TopicLogs.Topic` | |
| `Warehousing.Data.Push` | `Consumer[P]` | a function of the schema in Lean; here the consumer of each schema has its own `P` |
| `Warehousing.Topics` | `TopicLogs` | the topics with their logs, as `Warehousing.Run` is given them |
| `Warehousing.Pushes` | `Database.Pushes` | one log for each partition of each topic; the database gives its writing end |
| `Warehousing` | `Warehousing`, `NewWarehousing` | |
| `Warehousing.consumer` | `Consumer`, `ConsumerOf`, `SchemaConsumer` | one for each schema: a second for a schema is refused; given for the schemas of the topics read, where Lean's is given for every schema |
| `Warehousing.run` | `Warehousing.Run` | it also sends each partition's pushes to the topic's table, which in Lean the wiring does (`Wiring.workspace`); a topic's table is found by its name, so two topics of one name are refused, and so is one log given for two partitions, where Lean's `run` takes any topics |
| `Warehousing.run_apply` | `TestRunApply` | |

### `Visualization.lean`

| Declaration | Go | Note |
|---|---|---|
| `Visualization.Data` | `Chart` | no type of its own: its topics are the two sets `Chart.Draw` is given, its `View` `Chart`'s parameter |
| `Visualization.Data.Clean` | `Chart.Draw`, `ReadSet` | the first set |
| `Visualization.Data.cleanTopic` | `ReadSet.Topics` | |
| `Visualization.Data.Model` | `Chart.Draw`, `ReadSet` | the second set; it holds no topic without the engine |
| `Visualization.Data.modelTopic` | `ReadSet.Topics` | |
| `Visualization.Data.View` | `Chart[View]` | |
| `Visualization.In` | `Chart.Draw` | the pair is the two sets |
| `Visualization` | `Chart` | |
| `Visualization.chart` | `Chart.Draw` | |

### `MarketModelEngine.lean`

| Declaration | Go | Note |
|---|---|---|
| `MarketModelEngine.Data` | `Worker` | no type of its own: its topics are the two sets `Worker.Work` is given |
| `MarketModelEngine.Data.In` | `Worker.Work`, `ReadSet` | the set to read |
| `MarketModelEngine.Data.inTopic` | `ReadSet.Topics` | |
| `MarketModelEngine.Data.Out` | `Worker.Work`, `AppendSet` | the set to append to |
| `MarketModelEngine.Data.outTopic` | `AppendSet.Topics` | |
| `MarketModelEngine` | `Worker` | |
| `MarketModelEngine.worker` | `Worker.Work` | |

### `Workspace.lean`

Lean's `Workspace` is any four components wired by topics; `Wiring.workspace` is the one the definition's components
make. Go's `Workspace` is that one alone, made by `Wiring.Workspace`: the rows of the general structure name what
stands for each of its fields in it.

| Declaration | Go | Note |
|---|---|---|
| `Workspace.Component` | `Component` | |
| `Workspace.Component.acquisition` | `ComponentAcquisition` | |
| `Workspace.Component.warehousing` | `ComponentWarehousing` | |
| `Workspace.Component.visualization` | `ComponentVisualization` | |
| `Workspace.Component.marketModelEngine` | `ComponentMarketModelEngine` | |
| `Workspace` | `Workspace` | the one a wiring makes, and no other |
| `Workspace.Input` | `Inputs` | the two the wiring has |
| `Workspace.InputMsg` | `Inputs[Src]`, `Inputs[Corr]` | |
| `Workspace.Topic` | `Workspace.Topics` | |
| `Workspace.topic` | `Workspace.Topics`, `AnyTopic` | |
| `Workspace.names_distinct` | `Wiring.Workspace`, `ErrSameName`, `TestNamesDistinct` | a proof in Lean; here a name twice is refused when the working space is made |
| `Workspace.Output` | `Database.Pushes`, `Workspace.Run` | the logs that leave: warehousing's pushes, and the chart, which `Workspace.Run` is given to append to |
| `Workspace.OutputMsg` | `Consumer[P]`, `Chart[View]` | |
| `Workspace.table` | `Workspace.Table`, `Wiring.Tables` | the table's name; the consumers' `Database` values are one database, their giver's to keep |
| `Workspace.acquisition` | `Workspace.Run`, `Acquisition.Run` | on the inputs alone; `Workspace.Run` runs what `Acquisition.Run` runs, its checks made when the working space is made |
| `Workspace.warehousing` | `Workspace.Run`, `Warehousing.Run` | on the topics alone; `Workspace.Run` runs what `Warehousing.Run` runs, its checks made when the working space is made |
| `Workspace.visualization` | `Workspace.Run`, `Chart.Draw` | on the topics alone |
| `Workspace.marketModelEngine` | `Workspace.Run`, `Worker.Work` | on the topics alone |
| `Workspace.Inputs` | `Inputs` | |
| `Workspace.Topics` | `Wiring.Topics`, `Engine.Topics` | all the topics, each with its logs |
| `Workspace.Outputs` | `Database`, `Workspace.Run` | as `Workspace.Output` |
| `Workspace.Own` | | not in code: what a component writes, as one pair. A part is given the writing ends of what it writes; no value stands for the pair |
| `Workspace.run` | `Workspace.Run` | acquisition is given the inputs alone, the others topics alone |
| `Workspace.network` | `Workspace.Run` | the parts at work, the topics' logs between them |
| `Workspace.system` | `Workspace.Run` | |
| `Workspace.topics` | | not in code as a value: the topics as one process of the inputs. What the topics' logs hold after a run is what the tests read |
| `Workspace.system_apply` | | not in code: a theorem about the least solution of the network's equations. A run runs the parts; that what the logs then hold is that solution rests on each part keeping Kahn's condition and on what is given from outside being what it is given as, which no test here holds for every part and every binding |
| `Workspace.topics_eq` | | not in code: as `Workspace.system_apply` |
| `Workspace.topics_least` | | not in code: as `Workspace.system_apply` |
| `Workspace.acquisition_topics` | `TestAcquisitionTopics` | of the working space a wiring makes |
| `Workspace.TableLog` | `Database.Pushes` | a log pushed into a table: the table's name, and the partition of the topic it is from |
| `Workspace.TableRow` | `Database` | a row is a message marked with the log it is from: the database's to keep |
| `Workspace.tableOf` | `Database` | a table holds the rows of every log pushed into it, in no order: the database's to keep. Logs of several schemas pushed into one table go through several `Database` values, which are one database by their giver's keeping |
| `Workspace.tableRows` | `Database` | as `Workspace.tableOf`, from the inputs |
| `Workspace.has_tableRows` | `Database` | what a database is held to; the tests' database in memory is made so, and no test here holds another to it |
| `Workspaces` | | not in code: this package makes one working space. Several side by side are for who runs them, and so are the two conditions |
| `Workspaces.space` | | not in code: as `Workspaces` |
| `Workspaces.topics_distinct` | | not in code: as `Workspaces`; within one working space, `ErrSameName` |
| `Workspaces.tables_distinct` | | not in code: as `Workspaces` |
| `Workspaces.system` | | not in code: as `Workspaces` |
| `Workspaces.system_apply` | | not in code: as `Workspaces` |

### `Wiring.lean`

| Declaration | Go | Note |
|---|---|---|
| `Wiring.Feed` | `Inputs` | |
| `Wiring.Feed.source` | `Inputs.Source` | |
| `Wiring.Feed.corrections` | `Inputs.Corrections` | |
| `Wiring.AcqTopic` | `AllTopics`, `Workspace.Topics` | every topic of every kind |
| `Wiring.acqTopic` | `TopicLogs.Topic` | |
| `Wiring.AcqTopics` | `AllTopics` | with their logs |
| `Wiring.CleanTopic` | `AllTopics.Clean`, `ErrNotClean` | a type in Lean; here a topic to read that is not one of these is refused when the working space is made |
| `Wiring.CleanTopic.toAcq` | | not in code: a clean topic is already one of acquisition's, an entry of `AllTopics.Clean`, and nothing converts it |
| `Wiring.warehousingData` | `Wiring.Warehousing`, `Workspace.Run` | warehousing is given all of acquisition's topics, of every kind |
| `Wiring.engineData` | `Engine` | |
| `Wiring.Engine` | `Engine` | |
| `Wiring.Engine.Clean` | `Engine.Clean` | |
| `Wiring.Engine.clean` | `Engine.Clean`, `ErrNotClean` | each is one of acquisition's clean topics, or the working space is refused; one named twice is given once |
| `Wiring.Engine.Out` | `Engine.Topics` | |
| `Wiring.Engine.outTopic` | `Engine.Topics`, `TopicLogs.Topic` | |
| `Wiring.Engine.names_distinct` | `Wiring.Workspace`, `ErrSameName`, `TestNamesDistinct` | |
| `Wiring.Engine.engine` | `Engine.Worker` | |
| `Wiring.engineOut` | `Wiring.Engine`, `Workspace.Topics` | none without the engine |
| `Wiring.engineTopic` | `Workspace.Topics` | |
| `Wiring.engineIn` | `cleanTopics` | the engine's clean topics, from acquisition's |
| `Wiring.nothing` | | not in code: a family over no index. A component that writes no topic is given none, and a set of no topic is the zero `ReadSet` or `AppendSet` |
| `Wiring.engineRun` | `Workspace.Run` | without the engine no worker is run |
| `Wiring.visualizationData` | `Wiring.Shown`, `Wiring.Chart` | |
| `Wiring` | `Wiring` | |
| `Wiring.A` | `Wiring[Src]`, `Wiring[Kline]`, `Wiring[Corr]`, `Wiring.Topics` | |
| `Wiring.acquisition` | `Wiring.Acquisition` | |
| `Wiring.acq_names` | `Wiring.Workspace`, `ErrSameName`, `TestNamesDistinct` | |
| `Wiring.Push` | `Consumer[P]` | |
| `Wiring.warehousing` | `Wiring.Warehousing` | |
| `Wiring.table` | `Wiring.Tables` | by the topic's name; a topic with no table is refused; several topics may go into one table, as `Workspace.tableOf` |
| `Wiring.engine` | `Wiring.Engine` | nil for none |
| `Wiring.cross_names` | `Wiring.Workspace`, `ErrSameName`, `TestNamesDistinct` | |
| `Wiring.Shown` | `Wiring.Shown` | |
| `Wiring.shown` | `Wiring.Shown`, `ErrNotClean` | as `Wiring.Engine.clean` |
| `Wiring.View` | `Wiring[View]` | |
| `Wiring.visualization` | `Wiring.Chart` | |
| `Wiring.InputMsg` | `Inputs[Src]`, `Inputs[Corr]` | |
| `Wiring.Topic` | `Workspace.Topics` | acquisition's and the engine's; the others write none |
| `Wiring.topic` | `Workspace.Topics` | |
| `Wiring.engineTopic_names` | `TestNamesDistinct` | |
| `Wiring.names_distinct` | `TestNamesDistinct`, `Wiring.Workspace` | |
| `Wiring.Output` | `Database.Pushes`, `Workspace.Run` | warehousing's pushes, one log for each partition of each of acquisition's topics, and the chart |
| `Wiring.OutputMsg` | `Consumer[P]`, `Wiring[View]` | |
| `Wiring.Inputs` | `Inputs` | |
| `Wiring.Topics` | `Wiring.Topics`, `Engine.Topics` | |
| `Wiring.feeds` | `Inputs`, `ErrSameInput` | the inputs are given side by side, and nothing converts them; they are two logs, and one cursor given as both is refused where that can be told |
| `Wiring.acquisitions` | `Acquisition.Run` | as `Workspace.acquisition` |
| `Wiring.acqPart` | `Workspace.acquired` | |
| `Wiring.enginePart` | `Workspace.model` | |
| `Wiring.visualizationIn` | `Workspace.shown`, `Workspace.model` | |
| `Wiring.pushes` | `Warehousing.Run` | as `Workspace.warehousing` |
| `Wiring.workspace` | `Wiring.Workspace` | |
| `Wiring.acquisition_topics` | `TestAcquisitionTopics` | |
| `Wiring.clean_topics` | `TestWiringCleanTopics` | |
| `Wiring.engine_topics` | `TestEngineTopics` | |
| `Wiring.warehousing_pushes` | `TestWarehousingPushes` | |
| `Wiring.table_rows` | `TestTableRows` | |
| `Wiring.visualization_chart` | `TestVisualizationChart` | |
| `Wiring.chart_without_engine` | `TestChartWithoutEngine` | |

## Changes

- **0.1.0**: the first release, on `qi-v2-platform-definition` 0.1.0 and Go 1.26, the standard library alone. The
  open parts of the four components as Go interfaces (acquisition's reader, cleaner and three publishers, the
  warehouse consumer, visualization's chart, the engine's worker); a topic's shape, the three kinds of data and the
  wiring of a working space built in; the table from each of the 163 declarations of the definition's seven files to
  its Go name, or to why it is not in code.
