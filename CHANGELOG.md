# Changelog

## [Unreleased](https://github.com/babylonchain/vigilante/tree/HEAD)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.8.0...HEAD)

**Merged pull requests:**

- chore: bump to Babylon v0.8.0 [\#203](https://github.com/babylonchain/vigilante/pull/203) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: handle the gap and overlap between bootstrapping and ongoing execution [\#202](https://github.com/babylonchain/vigilante/pull/202) ([SebastianElvis](https://github.com/SebastianElvis))
- CI: Remove redundant SSH key logic [\#200](https://github.com/babylonchain/vigilante/pull/200) ([filippos47](https://github.com/filippos47))
- chore: Migrate private commits [\#194](https://github.com/babylonchain/vigilante/pull/194) ([gitferry](https://github.com/gitferry))
- chore: Refactor fee estimator [\#192](https://github.com/babylonchain/vigilante/pull/192) ([gitferry](https://github.com/gitferry))
- feat: Add multiplier for calculating tx fee of resending checkpoints [\#190](https://github.com/babylonchain/vigilante/pull/190) ([gitferry](https://github.com/gitferry))
- Dockerfile: Checkout only when explicitly asked to [\#189](https://github.com/babylonchain/vigilante/pull/189) ([filippos47](https://github.com/filippos47))
- fix: issue with multiple wallets [\#188](https://github.com/babylonchain/vigilante/pull/188) ([gitferry](https://github.com/gitferry))
- chore: Add more info to error [\#187](https://github.com/babylonchain/vigilante/pull/187) ([gitferry](https://github.com/gitferry))
- chore\(deps\): bump google.golang.org/grpc from 1.18.0 to 1.53.0 in /tools [\#186](https://github.com/babylonchain/vigilante/pull/186) ([dependabot[bot]](https://github.com/apps/dependabot))
- chore: Add more logs to show more information in fee estimation [\#185](https://github.com/babylonchain/vigilante/pull/185) ([gitferry](https://github.com/gitferry))
- chore: Add logs for estimating tx fees [\#184](https://github.com/babylonchain/vigilante/pull/184) ([gitferry](https://github.com/gitferry))
- Fix: fix metrics format [\#183](https://github.com/babylonchain/vigilante/pull/183) ([gitferry](https://github.com/gitferry))
- feat: Add BTC related metrics [\#182](https://github.com/babylonchain/vigilante/pull/182) ([gitferry](https://github.com/gitferry))
- fix: Submitter/Fix bumping fee [\#181](https://github.com/babylonchain/vigilante/pull/181) ([gitferry](https://github.com/gitferry))
- chore: Remove duplicate log and change log types [\#179](https://github.com/babylonchain/vigilante/pull/179) ([vitsalis](https://github.com/vitsalis))
- Use Replace-by-Fee if the previous sent checkpoint has not been included for long time [\#176](https://github.com/babylonchain/vigilante/pull/176) ([gitferry](https://github.com/gitferry))
- Babylon v0.7.1 [\#173](https://github.com/babylonchain/vigilante/pull/173) ([vitsalis](https://github.com/vitsalis))
- e2e tests [\#171](https://github.com/babylonchain/vigilante/pull/171) ([KonradStaniec](https://github.com/KonradStaniec))
- feat: Enable Replace By Fee [\#170](https://github.com/babylonchain/vigilante/pull/170) ([vitsalis](https://github.com/vitsalis))
- CI: Build and push images to ECR [\#169](https://github.com/babylonchain/vigilante/pull/169) ([filippos47](https://github.com/filippos47))

## [v0.8.0](https://github.com/babylonchain/vigilante/tree/v0.8.0) (2024-02-08)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.7.0...v0.8.0)

**Implemented enhancements:**

- Waste of previous BTC checkpoint transactions when new UTXOs are spent [\#175](https://github.com/babylonchain/vigilante/issues/175)

**Closed issues:**

- reporter: Bootstrap process keeps restarting after sync [\#201](https://github.com/babylonchain/vigilante/issues/201)

## [v0.7.0](https://github.com/babylonchain/vigilante/tree/v0.7.0) (2023-06-07)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.5.3...v0.7.0)

## [v0.5.3](https://github.com/babylonchain/vigilante/tree/v0.5.3) (2023-05-19)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.6.0...v0.5.3)

## [v0.6.0](https://github.com/babylonchain/vigilante/tree/v0.6.0) (2023-05-12)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.6.0-rc0...v0.6.0)

**Merged pull requests:**

- chore: Babylon v0.6.0 [\#168](https://github.com/babylonchain/vigilante/pull/168) ([vitsalis](https://github.com/vitsalis))

## [v0.6.0-rc0](https://github.com/babylonchain/vigilante/tree/v0.6.0-rc0) (2023-05-08)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.5.2...v0.6.0-rc0)

**Merged pull requests:**

- chore: Bump Babylon dependency and update Docker image [\#164](https://github.com/babylonchain/vigilante/pull/164) ([vitsalis](https://github.com/vitsalis))

## [v0.5.2](https://github.com/babylonchain/vigilante/tree/v0.5.2) (2023-05-04)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.5.1...v0.5.2)

**Implemented enhancements:**

- Submitter: add subscriber to Babylond node for Sealed checkpoints [\#54](https://github.com/babylonchain/vigilante/issues/54)
- Submitter: add Babylon submitter address [\#49](https://github.com/babylonchain/vigilante/issues/49)
- Submitter: finding smarter algorithm for retrieving unspent transactions [\#43](https://github.com/babylonchain/vigilante/issues/43)

**Closed issues:**

- Monitor: Running out of memory when bootstrapping from far behind [\#153](https://github.com/babylonchain/vigilante/issues/153)
- Duplicate checkpoint delays the reporting of the rest of the checkpoints [\#122](https://github.com/babylonchain/vigilante/issues/122)
- Generate new SegWit Bech32 change addresses for each submission after bumping btcd to v0.23.0 [\#107](https://github.com/babylonchain/vigilante/issues/107)

**Merged pull requests:**

- chore: circleci: Use go orb [\#166](https://github.com/babylonchain/vigilante/pull/166) ([vitsalis](https://github.com/vitsalis))
- monitor: fix: Do not use reported checkpoint if there's an error [\#165](https://github.com/babylonchain/vigilante/pull/165) ([vitsalis](https://github.com/vitsalis))
- chore: update deps with security issues [\#163](https://github.com/babylonchain/vigilante/pull/163) ([vitsalis](https://github.com/vitsalis))
- fix: fix potential memory leak in BTCCache [\#160](https://github.com/babylonchain/vigilante/pull/160) ([SebastianElvis](https://github.com/SebastianElvis))
- fix: Fix the issue of the monitor thread blocking the start of the rpc server [\#159](https://github.com/babylonchain/vigilante/pull/159) ([gitferry](https://github.com/gitferry))
- feat: Add babylon-related metrics [\#158](https://github.com/babylonchain/vigilante/pull/158) ([gitferry](https://github.com/gitferry))
- chore: Monitor/Optimize memory usage by removing confirmed blocks out of cache during bootstrapping [\#157](https://github.com/babylonchain/vigilante/pull/157) ([gitferry](https://github.com/gitferry))
- fix: Monitor/Fix out-of-memory bug when the bootstrapping is far behind [\#155](https://github.com/babylonchain/vigilante/pull/155) ([gitferry](https://github.com/gitferry))
- chore: Bump rpc-client version [\#152](https://github.com/babylonchain/vigilante/pull/152) ([gitferry](https://github.com/gitferry))
- fix: pprof support and reduce mem usage of reporter [\#151](https://github.com/babylonchain/vigilante/pull/151) ([SebastianElvis](https://github.com/SebastianElvis))
- chore: Add metric server config [\#150](https://github.com/babylonchain/vigilante/pull/150) ([gitferry](https://github.com/gitferry))
- fix: Fix bugs when testing with bitcoind regtest [\#149](https://github.com/babylonchain/vigilante/pull/149) ([gitferry](https://github.com/gitferry))
- hotfix: Continue submitting checkpoints from the queue if one of them fails [\#148](https://github.com/babylonchain/vigilante/pull/148) ([vitsalis](https://github.com/vitsalis))
- feat: Add monitor dockerization and remove private build [\#146](https://github.com/babylonchain/vigilante/pull/146) ([gitferry](https://github.com/gitferry))

## [v0.5.1](https://github.com/babylonchain/vigilante/tree/v0.5.1) (2023-02-07)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.5.0...v0.5.1)

**Implemented enhancements:**

- Submitter: hardcoded txfee [\#42](https://github.com/babylonchain/vigilante/issues/42)

**Closed issues:**

- reporter: Reporter running out of memory after a few days of running on the mainnet with 1GB RAM [\#88](https://github.com/babylonchain/vigilante/issues/88)

## [v0.5.0](https://github.com/babylonchain/vigilante/tree/v0.5.0) (2023-02-03)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.4.0...v0.5.0)

**Merged pull requests:**

- feat: Add monitor mode [\#145](https://github.com/babylonchain/vigilante/pull/145) ([vitsalis](https://github.com/vitsalis))
- Bump rpc-client to v0.5.0 and Babylon to v0.5.0 [\#144](https://github.com/babylonchain/vigilante/pull/144) ([vitsalis](https://github.com/vitsalis))
- Release v0.5.0 [\#143](https://github.com/babylonchain/vigilante/pull/143) ([vitsalis](https://github.com/vitsalis))
- chore: Monitor/Clean up the vigilante monitor [\#140](https://github.com/babylonchain/vigilante/pull/140) ([gitferry](https://github.com/gitferry))
- Change to Apache 2.0 license [\#139](https://github.com/babylonchain/vigilante/pull/139) ([vitsalis](https://github.com/vitsalis))
- Bump Babylon dependency [\#138](https://github.com/babylonchain/vigilante/pull/138) ([vitsalis](https://github.com/vitsalis))
- fix: Monitor/Fix bugs in handling new BTC blocks [\#137](https://github.com/babylonchain/vigilante/pull/137) ([gitferry](https://github.com/gitferry))
- fix: Monitor/Fix deadlock bugs in bootstrapping [\#136](https://github.com/babylonchain/vigilante/pull/136) ([gitferry](https://github.com/gitferry))
- chore: Monitor/Add stop mechanism and fix linter [\#135](https://github.com/babylonchain/vigilante/pull/135) ([gitferry](https://github.com/gitferry))
- feat: Monitor/Add liveness attack detection [\#134](https://github.com/babylonchain/vigilante/pull/134) ([gitferry](https://github.com/gitferry))
- Bump btcd versions to fix 2 consensus issues [\#133](https://github.com/babylonchain/vigilante/pull/133) ([KonradStaniec](https://github.com/KonradStaniec))
- feat: Monitor/Add liveness checker [\#132](https://github.com/babylonchain/vigilante/pull/132) ([gitferry](https://github.com/gitferry))
- feat: Monitor/Add block handler [\#131](https://github.com/babylonchain/vigilante/pull/131) ([gitferry](https://github.com/gitferry))
- feat: Monitor/Add bootstrap process to scanner [\#130](https://github.com/babylonchain/vigilante/pull/130) ([gitferry](https://github.com/gitferry))
- chore: Add aggressive linting with golangci-lint [\#129](https://github.com/babylonchain/vigilante/pull/129) ([vitsalis](https://github.com/vitsalis))
- chore: move verifier logic to monitor [\#128](https://github.com/babylonchain/vigilante/pull/128) ([gitferry](https://github.com/gitferry))
- Bump golang to 1.19 [\#127](https://github.com/babylonchain/vigilante/pull/127) ([vitsalis](https://github.com/vitsalis))
- hotfix: Fix fee rate bug [\#126](https://github.com/babylonchain/vigilante/pull/126) ([gitferry](https://github.com/gitferry))
- hotfix: Fix fee rate bug [\#125](https://github.com/babylonchain/vigilante/pull/125) ([gitferry](https://github.com/gitferry))

## [v0.4.0](https://github.com/babylonchain/vigilante/tree/v0.4.0) (2022-12-22)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.3.0...v0.4.0)

**Closed issues:**

- Submitter: error while submitting checkpoint to BTC if address is segwit [\#111](https://github.com/babylonchain/vigilante/issues/111)

**Merged pull requests:**

- chore: changed fee estimation failure from panic to default tx fee [\#124](https://github.com/babylonchain/vigilante/pull/124) ([gitferry](https://github.com/gitferry))
- Release v0.4.0 [\#123](https://github.com/babylonchain/vigilante/pull/123) ([vitsalis](https://github.com/vitsalis))
- chore: bump rpc-client to v0.2.0 [\#121](https://github.com/babylonchain/vigilante/pull/121) ([gitferry](https://github.com/gitferry))
- chore: Replace babylonclient with rpc-client [\#120](https://github.com/babylonchain/vigilante/pull/120) ([gitferry](https://github.com/gitferry))
- feat: add witness signature for segwit tx [\#119](https://github.com/babylonchain/vigilante/pull/119) ([gitferry](https://github.com/gitferry))
- feat: add fee estimation [\#118](https://github.com/babylonchain/vigilante/pull/118) ([gitferry](https://github.com/gitferry))
- submitter: use listunspent rpc api [\#115](https://github.com/babylonchain/vigilante/pull/115) ([gusin13](https://github.com/gusin13))

## [v0.3.0](https://github.com/babylonchain/vigilante/tree/v0.3.0) (2022-11-22)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.2.0...v0.3.0)

**Merged pull requests:**

- Release v0.3.0 [\#116](https://github.com/babylonchain/vigilante/pull/116) ([gusin13](https://github.com/gusin13))

## [v0.2.0](https://github.com/babylonchain/vigilante/tree/v0.2.0) (2022-11-17)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.1.2...v0.2.0)

**Merged pull requests:**

- Release v0.2.0 [\#114](https://github.com/babylonchain/vigilante/pull/114) ([vitsalis](https://github.com/vitsalis))
- hotfix: Do not panic on an invalid checkpoint [\#112](https://github.com/babylonchain/vigilante/pull/112) ([gusin13](https://github.com/gusin13))
- submitter: support bitcoind backend [\#110](https://github.com/babylonchain/vigilante/pull/110) ([gusin13](https://github.com/gusin13))
- chore: Add add\_ssh\_keys step in CircleCI [\#109](https://github.com/babylonchain/vigilante/pull/109) ([vitsalis](https://github.com/vitsalis))
- Release v0.1.1 [\#105](https://github.com/babylonchain/vigilante/pull/105) ([vitsalis](https://github.com/vitsalis))
- feat: choose change address from local addresses and prefer SegWit Bech32 type to reduce tx fee [\#103](https://github.com/babylonchain/vigilante/pull/103) ([gitferry](https://github.com/gitferry))
- fix: Install zmq libraries on the Docker images [\#102](https://github.com/babylonchain/vigilante/pull/102) ([vitsalis](https://github.com/vitsalis))
- reporter: fuzz tests for btccache [\#98](https://github.com/babylonchain/vigilante/pull/98) ([gusin13](https://github.com/gusin13))

## [v0.1.2](https://github.com/babylonchain/vigilante/tree/v0.1.2) (2022-11-10)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.1.1...v0.1.2)

**Merged pull requests:**

- hotfix: Do not panic on an invalid checkpoint [\#106](https://github.com/babylonchain/vigilante/pull/106) ([vitsalis](https://github.com/vitsalis))

## [v0.1.1](https://github.com/babylonchain/vigilante/tree/v0.1.1) (2022-11-09)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/v0.1.0...v0.1.1)

**Merged pull requests:**

- hotfix: Fix TooFewEntries bug on cache [\#104](https://github.com/babylonchain/vigilante/pull/104) ([vitsalis](https://github.com/vitsalis))

## [v0.1.0](https://github.com/babylonchain/vigilante/tree/v0.1.0) (2022-11-07)

[Full Changelog](https://github.com/babylonchain/vigilante/compare/d4a48f931a6631ebcdece9c7fc6fa732ba81f901...v0.1.0)

**Implemented enhancements:**

- TestTag input is not human-readable and not configurable [\#53](https://github.com/babylonchain/vigilante/issues/53)
- Submitter: statefull submitter to avoid resending checkpoints for the same epoch [\#48](https://github.com/babylonchain/vigilante/issues/48)
- Submitter: investigate the possibility of chaining two BTC txs of a checkpoint via input [\#44](https://github.com/babylonchain/vigilante/issues/44)

**Fixed bugs:**

- Testnet error: insufficient priority when sending BTC tx [\#58](https://github.com/babylonchain/vigilante/issues/58)
- Submitter: failed to send BTC txs due to referencing orphan txs [\#40](https://github.com/babylonchain/vigilante/issues/40)
- reporter: BTC header submission to Babylon stalls if it passes verification  [\#26](https://github.com/babylonchain/vigilante/issues/26)
- reporter: Sync fails if there are existing blocks in the BTC chain [\#25](https://github.com/babylonchain/vigilante/issues/25)
- reporter: Error when submitting BTC headers [\#24](https://github.com/babylonchain/vigilante/issues/24)

**Closed issues:**

- submitter: Only the earliest epoch sealed checkpoint is accepted if included in the same BTC block [\#76](https://github.com/babylonchain/vigilante/issues/76)
- reporter: bootstrapping: Submit multiple headers at once [\#60](https://github.com/babylonchain/vigilante/issues/60)
- Reporter: Investigate cause behind "account sequence mismatch error" [\#46](https://github.com/babylonchain/vigilante/issues/46)

**Merged pull requests:**

- Release/0.1.0 [\#101](https://github.com/babylonchain/vigilante/pull/101) ([vitsalis](https://github.com/vitsalis))
- babylonclient: fuzz test for keys [\#96](https://github.com/babylonchain/vigilante/pull/96) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: refactor reporter, datagen and fuzz tests on block handlers [\#95](https://github.com/babylonchain/vigilante/pull/95) ([SebastianElvis](https://github.com/SebastianElvis))
- test: fuzz tests on IndexedBlock [\#94](https://github.com/babylonchain/vigilante/pull/94) ([SebastianElvis](https://github.com/SebastianElvis))
- test: refactor and fuzz tests for checkpoint segments and pools [\#93](https://github.com/babylonchain/vigilante/pull/93) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: setup zmq block subscription [\#92](https://github.com/babylonchain/vigilante/pull/92) ([gusin13](https://github.com/gusin13))
- Fix deadlock due to incorrect mutex [\#91](https://github.com/babylonchain/vigilante/pull/91) ([gusin13](https://github.com/gusin13))
- Modularize reporter bootstrapping process [\#90](https://github.com/babylonchain/vigilante/pull/90) ([gusin13](https://github.com/gusin13))
- chore: submitter/improve the relayer [\#89](https://github.com/babylonchain/vigilante/pull/89) ([gitferry](https://github.com/gitferry))
- chore: submitter/refactor poller [\#87](https://github.com/babylonchain/vigilante/pull/87) ([gitferry](https://github.com/gitferry))
- chore: Update Babylon version and bond denomination [\#85](https://github.com/babylonchain/vigilante/pull/85) ([vitsalis](https://github.com/vitsalis))
- reporter: handle re-orgs [\#84](https://github.com/babylonchain/vigilante/pull/84) ([gusin13](https://github.com/gusin13))
- fix: Trim cache based on a valid slice [\#81](https://github.com/babylonchain/vigilante/pull/81) ([vitsalis](https://github.com/vitsalis))
- chore: fix mock file directory in Makefile [\#80](https://github.com/babylonchain/vigilante/pull/80) ([SebastianElvis](https://github.com/SebastianElvis))
- Move retry module from Vigilante to BBN [\#79](https://github.com/babylonchain/vigilante/pull/79) ([gusin13](https://github.com/gusin13))
- Move retry config out of reporter so its accessible by other clients [\#78](https://github.com/babylonchain/vigilante/pull/78) ([gusin13](https://github.com/gusin13))
- feat: Submitter/chaining utxo [\#75](https://github.com/babylonchain/vigilante/pull/75) ([gitferry](https://github.com/gitferry))
- Allow the usage of indexed mainnet tags [\#74](https://github.com/babylonchain/vigilante/pull/74) ([vitsalis](https://github.com/vitsalis))
- reporter: improving resilience [\#69](https://github.com/babylonchain/vigilante/pull/69) ([SebastianElvis](https://github.com/SebastianElvis))
- fix: Submit proofs in order of their epoch number [\#68](https://github.com/babylonchain/vigilante/pull/68) ([vitsalis](https://github.com/vitsalis))
- hotfix: bugs in submitheaders [\#66](https://github.com/babylonchain/vigilante/pull/66) ([SebastianElvis](https://github.com/SebastianElvis))
- feat: submitter/add sent checkpoints [\#64](https://github.com/babylonchain/vigilante/pull/64) ([gitferry](https://github.com/gitferry))
- reporter: submit headers with deduplication [\#63](https://github.com/babylonchain/vigilante/pull/63) ([SebastianElvis](https://github.com/SebastianElvis))
- hotfix: change utxo list order [\#62](https://github.com/babylonchain/vigilante/pull/62) ([gitferry](https://github.com/gitferry))
- hotfix: Submitting multiple headers fails if one is a duplicate [\#61](https://github.com/babylonchain/vigilante/pull/61) ([vitsalis](https://github.com/vitsalis))
- fix: change balance bug [\#59](https://github.com/babylonchain/vigilante/pull/59) ([gitferry](https://github.com/gitferry))
- feat: add configurable tag idx [\#57](https://github.com/babylonchain/vigilante/pull/57) ([gitferry](https://github.com/gitferry))
- fix: Use BTC network name instead of configuration one [\#55](https://github.com/babylonchain/vigilante/pull/55) ([vitsalis](https://github.com/vitsalis))
- btcclient: poll-based BTC client [\#52](https://github.com/babylonchain/vigilante/pull/52) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: add exponential backoff in retry mechanism and other fixes [\#50](https://github.com/babylonchain/vigilante/pull/50) ([gusin13](https://github.com/gusin13))
- reporter: bugfix of extracting ckpts [\#47](https://github.com/babylonchain/vigilante/pull/47) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: add retry mechanism in header and checkpoint submission [\#45](https://github.com/babylonchain/vigilante/pull/45) ([gusin13](https://github.com/gusin13))
- reporter: fix trimming cache size [\#39](https://github.com/babylonchain/vigilante/pull/39) ([SebastianElvis](https://github.com/SebastianElvis))
- reporter: use temp `QueryContainsBytes` API and fix stalling issue [\#38](https://github.com/babylonchain/vigilante/pull/38) ([SebastianElvis](https://github.com/SebastianElvis))
- feat: submitter/send sealed checkpoints to BTC [\#37](https://github.com/babylonchain/vigilante/pull/37) ([gitferry](https://github.com/gitferry))
- Add instructions for creating a wallet and corresponding config entries [\#34](https://github.com/babylonchain/vigilante/pull/34) ([vitsalis](https://github.com/vitsalis))
- reporter: multiple fixes on resolving corner cases [\#33](https://github.com/babylonchain/vigilante/pull/33) ([SebastianElvis](https://github.com/SebastianElvis))
- doc: more fixes on documentations [\#32](https://github.com/babylonchain/vigilante/pull/32) ([SebastianElvis](https://github.com/SebastianElvis))
- chore: vanilla test files [\#31](https://github.com/babylonchain/vigilante/pull/31) ([SebastianElvis](https://github.com/SebastianElvis))
- ci: vanilla tests and CI [\#30](https://github.com/babylonchain/vigilante/pull/30) ([SebastianElvis](https://github.com/SebastianElvis))
- chore: fix inconsistency of env variable names in README [\#29](https://github.com/babylonchain/vigilante/pull/29) ([SebastianElvis](https://github.com/SebastianElvis))
- Add GOPRIVATE info in Readme [\#28](https://github.com/babylonchain/vigilante/pull/28) ([gusin13](https://github.com/gusin13))
- fix: ModuleBasics not included when bootstrapping from config file [\#27](https://github.com/babylonchain/vigilante/pull/27) ([vitsalis](https://github.com/vitsalis))
- fix: Change .yaml to .yml [\#23](https://github.com/babylonchain/vigilante/pull/23) ([vitsalis](https://github.com/vitsalis))
- fix: Clarify local build instructions [\#22](https://github.com/babylonchain/vigilante/pull/22) ([vitsalis](https://github.com/vitsalis))
- fix: Add submitter netparams to sample configs [\#21](https://github.com/babylonchain/vigilante/pull/21) ([vitsalis](https://github.com/vitsalis))
- chore: default account prefix [\#20](https://github.com/babylonchain/vigilante/pull/20) ([SebastianElvis](https://github.com/SebastianElvis))
- fix: Use proper Babylon directory on Docker images [\#19](https://github.com/babylonchain/vigilante/pull/19) ([vitsalis](https://github.com/vitsalis))
- Update Babylon version and minor fixes [\#18](https://github.com/babylonchain/vigilante/pull/18) ([vitsalis](https://github.com/vitsalis))
- reporter: consistency check and extract/forward ckpts during bootstrapping [\#17](https://github.com/babylonchain/vigilante/pull/17) ([SebastianElvis](https://github.com/SebastianElvis))
- bootstrapping: download BTC blocks and forward headers to BBN [\#15](https://github.com/babylonchain/vigilante/pull/15) ([gusin13](https://github.com/gusin13))
- docker: Docker image for submitter and reporter. Updated instructions [\#14](https://github.com/babylonchain/vigilante/pull/14) ([vitsalis](https://github.com/vitsalis))
- submitter: separate constructor functions for BTC client [\#13](https://github.com/babylonchain/vigilante/pull/13) ([SebastianElvis](https://github.com/SebastianElvis))
- submitter: query ckpt APIs and poller [\#12](https://github.com/babylonchain/vigilante/pull/12) ([SebastianElvis](https://github.com/SebastianElvis))
- bootstrapping: moving the initialisation of BTC cache to reporter [\#11](https://github.com/babylonchain/vigilante/pull/11) ([SebastianElvis](https://github.com/SebastianElvis))
- doc: documentations on the private dependency issue [\#10](https://github.com/babylonchain/vigilante/pull/10) ([SebastianElvis](https://github.com/SebastianElvis))
- Bootstrapping - Sync latest BTC blocks and store in memory  [\#9](https://github.com/babylonchain/vigilante/pull/9) ([gusin13](https://github.com/gusin13))
- babylonclient: extract and forward checkpoints [\#8](https://github.com/babylonchain/vigilante/pull/8) ([SebastianElvis](https://github.com/SebastianElvis))
- chore: update documentation [\#7](https://github.com/babylonchain/vigilante/pull/7) ([SebastianElvis](https://github.com/SebastianElvis))
- babylon: vanilla code for submitting txs to Babylon [\#6](https://github.com/babylonchain/vigilante/pull/6) ([SebastianElvis](https://github.com/SebastianElvis))
- btcclient: subscribing and handling new BTC blocks [\#5](https://github.com/babylonchain/vigilante/pull/5) ([SebastianElvis](https://github.com/SebastianElvis))
- docker: Dockerfile [\#4](https://github.com/babylonchain/vigilante/pull/4) ([SebastianElvis](https://github.com/SebastianElvis))
- chore: remove btc client dependency in submitter [\#3](https://github.com/babylonchain/vigilante/pull/3) ([SebastianElvis](https://github.com/SebastianElvis))
- Vanilla Babylon client implementation [\#2](https://github.com/babylonchain/vigilante/pull/2) ([SebastianElvis](https://github.com/SebastianElvis))
- vanilla vigilante codebase [\#1](https://github.com/babylonchain/vigilante/pull/1) ([SebastianElvis](https://github.com/SebastianElvis))



\* *This Changelog was automatically generated by [github_changelog_generator](https://github.com/github-changelog-generator/github-changelog-generator)*
