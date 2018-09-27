package store_test

import (
	"errors"
	"fmt"
	"policy-server/store"
	"policy-server/store/fakes"
	"test-helpers"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dbfakes "code.cloudfoundry.org/cf-networking-helpers/db/fakes"

	"code.cloudfoundry.org/cf-networking-helpers/db"
	"code.cloudfoundry.org/cf-networking-helpers/testsupport"
	"code.cloudfoundry.org/lager"
)

var _ = Describe("Egress Policy Table", func() {
	var (
		dbConf            db.Config
		realDb            *db.ConnWrapper
		mockDb            *fakes.Db
		egressPolicyTable *store.EgressPolicyTable
		terminalsTable    *store.TerminalsTable
		tx                db.Transaction
		fakeGUIDGenerator *fakes.GUIDGenerator
	)

	getMigratedRealDb := func(dbConfig db.Config) (*db.ConnWrapper, db.Transaction) {
		var err error
		testhelpers.CreateDatabase(dbConf)

		logger := lager.NewLogger("Egress Store Test")

		realDb, err = db.NewConnectionPool(dbConf, 200, 200, 5*time.Minute, "Egress Store Test", "Egress Store Test", logger)
		Expect(err).NotTo(HaveOccurred())

		migrate(realDb)
		tx, err = realDb.Beginx()
		Expect(err).NotTo(HaveOccurred())
		return realDb, tx
	}

	setupEgressPolicyStore := func(db store.Database) store.EgressPolicyStore {
		var currentGUID = 0
		fakeGUIDGenerator = &fakes.GUIDGenerator{}
		fakeGUIDGenerator.NewStub = func() string {
			currentGUID++
			return fmt.Sprintf("guid-%d", currentGUID)
		}

		terminalsTable = &store.TerminalsTable{
			Guids: &store.GuidGenerator{},
		}

		egressPolicyTable = &store.EgressPolicyTable{
			Conn:  db,
			Guids: fakeGUIDGenerator,
		}
		return store.EgressPolicyStore{
			EgressPolicyRepo: egressPolicyTable,
			TerminalsRepo:    terminalsTable,
			Conn:             db,
		}
	}

	BeforeEach(func() {
		mockDb = &fakes.Db{}

		dbConf = testsupport.GetDBConfig()
		dbConf.DatabaseName = fmt.Sprintf("store_test_node_%d", time.Now().UnixNano())
		dbConf.Timeout = 30
	})

	AfterEach(func() {
		if tx != nil {
			tx.Rollback()
		}
		if realDb != nil {
			Expect(realDb.Close()).To(Succeed())
		}
		testhelpers.RemoveDatabase(dbConf)
	})

	Context("CreateApp", func() {
		It("should create an app and return the ID", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			appTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			id, err := egressPolicyTable.CreateApp(tx, appTerminalGUID, "some-app-guid")
			Expect(err).ToNot(HaveOccurred())

			Expect(id).To(Equal(int64(1)))

			var foundAppGuid string
			row := tx.QueryRow(`SELECT app_guid FROM apps WHERE id = 1`)
			err = row.Scan(&foundAppGuid)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundAppGuid).To(Equal("some-app-guid"))
		})

		It("should return an error if the driver is not supported", func() {
			setupEgressPolicyStore(mockDb)
			fakeTx := &dbfakes.Transaction{}

			fakeTx.DriverNameReturns("db2")

			_, err := egressPolicyTable.CreateApp(fakeTx, "some-term-guid", "some-app-guid")
			Expect(err).To(MatchError("unknown driver: db2"))
		})
	})

	Context("CreateSpace", func() {
		It("should create a space and return the ID", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			spaceTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			id, err := egressPolicyTable.CreateSpace(tx, spaceTerminalGUID, "some-space-guid")
			Expect(err).ToNot(HaveOccurred())

			Expect(id).To(Equal(int64(1)))

			var foundSpaceGuid string
			row := tx.QueryRow(`SELECT space_guid FROM spaces WHERE id = 1`)
			err = row.Scan(&foundSpaceGuid)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundSpaceGuid).To(Equal("some-space-guid"))
		})

		It("should return an error if the driver is not supported", func() {
			setupEgressPolicyStore(mockDb)
			fakeTx := &dbfakes.Transaction{}

			fakeTx.DriverNameReturns("db2")
			_, err := egressPolicyTable.CreateSpace(fakeTx, "some-term-guid", "some-space-guid")
			Expect(err).To(MatchError("unknown driver: db2"))
		})
	})

	Context("CreateIPRange", func() {
		It("should create an iprange and return the ID", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			ipRangeTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			id, err := egressPolicyTable.CreateIPRange(tx, ipRangeTerminalGUID, "1.1.1.1", "2.2.2.2", "tcp", 8080, 8081, 0, 0)
			Expect(err).ToNot(HaveOccurred())

			Expect(id).To(Equal(int64(1)))

			var startIP, endIP, protocol string
			var startPort, endPort, icmpType, icmpCode int64
			row := tx.QueryRow(`SELECT start_ip, end_ip, protocol, start_port, end_port, icmp_type, icmp_code FROM ip_ranges WHERE id = 1`)
			err = row.Scan(&startIP, &endIP, &protocol, &startPort, &endPort, &icmpType, &icmpCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(startPort).To(Equal(int64(8080)))
			Expect(endPort).To(Equal(int64(8081)))
			Expect(startIP).To(Equal("1.1.1.1"))
			Expect(endIP).To(Equal("2.2.2.2"))
			Expect(protocol).To(Equal("tcp"))
			Expect(icmpType).To(Equal(int64(0)))
			Expect(icmpCode).To(Equal(int64(0)))
		})

		It("should create an iprange with icmp and return the ID", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			ipRangeTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			id, err := egressPolicyTable.CreateIPRange(tx, ipRangeTerminalGUID, "1.1.1.1", "2.2.2.2", "icmp", 0, 0, 2, 1)
			Expect(err).ToNot(HaveOccurred())

			Expect(id).To(Equal(int64(1)))

			var startIP, endIP, protocol string
			var startPort, endPort, icmpType, icmpCode int64
			row := tx.QueryRow(`SELECT start_ip, end_ip, protocol, start_port, end_port, icmp_type, icmp_code FROM ip_ranges WHERE id = 1`)
			err = row.Scan(&startIP, &endIP, &protocol, &startPort, &endPort, &icmpType, &icmpCode)
			Expect(err).ToNot(HaveOccurred())
			Expect(startPort).To(Equal(int64(0)))
			Expect(endPort).To(Equal(int64(0)))
			Expect(startIP).To(Equal("1.1.1.1"))
			Expect(endIP).To(Equal("2.2.2.2"))
			Expect(protocol).To(Equal("icmp"))
			Expect(icmpType).To(Equal(int64(2)))
			Expect(icmpCode).To(Equal(int64(1)))
		})

		It("should return an error if the driver is not supported", func() {
			setupEgressPolicyStore(mockDb)
			fakeTx := &dbfakes.Transaction{}

			fakeTx.DriverNameReturns("db2")

			_, err := egressPolicyTable.CreateIPRange(fakeTx, "some-term-guid", "1.1.1.1", "2.2.2.2", "tcp", 8080, 8081, 0, 0)
			Expect(err).To(MatchError("unknown driver: db2"))
		})
	})

	Context("CreateEgressPolicy", func() {
		It("should create and return the id for an egress policy", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			sourceTerminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			destinationTerminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			guid, err := egressPolicyTable.CreateEgressPolicy(tx, sourceTerminalId, destinationTerminalId)
			Expect(err).ToNot(HaveOccurred())
			Expect(guid).To(Equal("guid-1"))

			var foundSourceID, foundDestinationID string
			row := tx.QueryRow(tx.Rebind(`SELECT source_guid, destination_guid FROM egress_policies WHERE guid = ?`), guid)
			err = row.Scan(&foundSourceID, &foundDestinationID)
			Expect(err).ToNot(HaveOccurred())
			Expect(foundSourceID).To(Equal(sourceTerminalId))
			Expect(foundDestinationID).To(Equal(destinationTerminalId))

			By("checking that if bad args are sent, it returns an error") // merged because db's are slow
			_, err = egressPolicyTable.CreateEgressPolicy(tx, "some-term-guid", "some-term-guid")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("DeleteEgressPolicy", func() {
		It("deletes the policy", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			sourceTerminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			destinationTerminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			egressPolicyGUID, err := egressPolicyTable.CreateEgressPolicy(tx, sourceTerminalId, destinationTerminalId)
			Expect(err).ToNot(HaveOccurred())

			err = egressPolicyTable.DeleteEgressPolicy(tx, egressPolicyGUID)
			Expect(err).ToNot(HaveOccurred())

			var policyCount int
			row := tx.QueryRow(tx.Rebind(`SELECT COUNT(guid) FROM egress_policies WHERE guid = ?`), egressPolicyGUID)
			err = row.Scan(&policyCount)
			Expect(err).ToNot(HaveOccurred())
			Expect(policyCount).To(Equal(0))
		})

		It("should return the sql error", func() {
			fakeTx := &dbfakes.Transaction{}
			fakeTx.ExecReturns(nil, errors.New("broke"))

			setupEgressPolicyStore(mockDb)

			err := egressPolicyTable.DeleteEgressPolicy(fakeTx, "some-guid")
			Expect(err).To(MatchError("broke"))
		})
	})

	Context("DeleteIPRange", func() {
		It("deletes the ip range", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			ipRangeTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			ipRangeID, err := egressPolicyTable.CreateIPRange(tx, ipRangeTerminalGUID, "1.1.1.1", "2.2.2.2", "tcp", 8080, 8081, 0, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipRangeID).To(Equal(int64(1)))

			err = egressPolicyTable.DeleteIPRange(tx, ipRangeID)
			Expect(err).ToNot(HaveOccurred())

			var ipRangeCount int
			row := tx.QueryRow(`SELECT COUNT(id) FROM ip_ranges WHERE id = 1`)
			err = row.Scan(&ipRangeCount)
			Expect(err).ToNot(HaveOccurred())
			Expect(ipRangeCount).To(Equal(0))
		})

		It("should return the sql error", func() {
			setupEgressPolicyStore(mockDb)

			fakeTx := &dbfakes.Transaction{}
			fakeTx.ExecReturns(nil, errors.New("broke"))

			err := egressPolicyTable.DeleteIPRange(fakeTx, 2)
			Expect(err).To(MatchError("broke"))
		})
	})

	Context("DeleteTerminal", func() {
		It("deletes the terminal", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			var err error
			terminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			err = terminalsTable.Delete(tx, terminalGUID)
			Expect(err).ToNot(HaveOccurred())

			var terminalCount int
			row := tx.QueryRow(tx.Rebind(`SELECT COUNT(guid) FROM terminals WHERE guid = ?`), terminalGUID)
			err = row.Scan(&terminalCount)
			Expect(err).ToNot(HaveOccurred())
			Expect(terminalCount).To(Equal(0))
		})

		It("should return the sql error", func() {
			setupEgressPolicyStore(mockDb)

			fakeTx := &dbfakes.Transaction{}
			fakeTx.ExecReturns(nil, errors.New("broke"))

			err := terminalsTable.Delete(fakeTx, "some-term-guid")
			Expect(err).To(MatchError("broke"))
		})
	})

	Context("DeleteApp", func() {
		It("deletes the app provided a terminal guid", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			appTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			appID, err := egressPolicyTable.CreateApp(tx, appTerminalGUID, "some-app-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(appID).To(Equal(int64(1)))

			err = egressPolicyTable.DeleteApp(tx, appTerminalGUID)
			Expect(err).ToNot(HaveOccurred())

			var appCount int
			row := tx.QueryRow(`SELECT COUNT(id) FROM apps WHERE id = 1`)
			err = row.Scan(&appCount)
			Expect(err).ToNot(HaveOccurred())
			Expect(appCount).To(Equal(0))
		})

		It("should return the sql error", func() {
			setupEgressPolicyStore(mockDb)

			fakeTx := &dbfakes.Transaction{}
			fakeTx.ExecReturns(nil, errors.New("broke"))

			err := egressPolicyTable.DeleteApp(fakeTx, "2")
			Expect(err).To(MatchError("broke"))
		})
	})

	Context("DeleteSpace", func() {
		It("deletes the space provided a terminal guid", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			spaceTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			spaceID, err := egressPolicyTable.CreateSpace(tx, spaceTerminalGUID, "some-space-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceID).To(Equal(int64(1)))

			err = egressPolicyTable.DeleteSpace(tx, spaceTerminalGUID)
			Expect(err).ToNot(HaveOccurred())

			var spaceCount int
			row := tx.QueryRow(`SELECT COUNT(id) FROM spaces WHERE id = 1`)
			err = row.Scan(&spaceCount)
			Expect(err).ToNot(HaveOccurred())
			Expect(spaceCount).To(Equal(0))
		})

		It("should return the sql error", func() {
			setupEgressPolicyStore(mockDb)

			fakeTx := &dbfakes.Transaction{}
			fakeTx.ExecReturns(nil, errors.New("broke"))

			err := egressPolicyTable.DeleteSpace(fakeTx, "a-guid")
			Expect(err).To(MatchError("broke"))
		})
	})

	Context("IsTerminalInUse", func() {
		It("returns true if the terminal is in use by an egress policy", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			destinationTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			sourceTerminalGUID, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())

			_, err = egressPolicyTable.CreateEgressPolicy(tx, sourceTerminalGUID, destinationTerminalGUID)
			Expect(err).ToNot(HaveOccurred())
			inUse, err := egressPolicyTable.IsTerminalInUse(tx, sourceTerminalGUID)
			Expect(err).ToNot(HaveOccurred())
			Expect(inUse).To(BeTrue())

			By("returns false if the terminal is not in use by an egress policy") //combined because db's are slow
			inUse, err = egressPolicyTable.IsTerminalInUse(tx, "some-term-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(inUse).To(BeFalse())
		})
	})

	Context("when retrieving egress policies", func() {
		var (
			egressPolicies            []store.EgressPolicy
			createdEgressPolicies     []store.EgressPolicy
			egressDestinations        []store.EgressDestination
			createdEgressDestinations []store.EgressDestination
		)

		BeforeEach(func() {
			db, _ := getMigratedRealDb(dbConf)
			egressStore := setupEgressPolicyStore(db)

			var err error

			egressDestinations = []store.EgressDestination{
				{
					Name:        "a",
					Description: "desc a",
					Protocol:    "tcp",
					Ports: []store.Ports{
						{
							Start: 8080,
							End:   8081,
						},
					},
					IPRanges: []store.IPRange{
						{
							Start: "1.2.3.4",
							End:   "1.2.3.5",
						},
					},
				},
				{
					Name:        "b",
					Description: "desc b",
					Protocol:    "udp",
					IPRanges: []store.IPRange{
						{
							Start: "2.2.3.4",
							End:   "2.2.3.5",
						},
					},
				},
				{
					Name:        "c",
					Description: "desc c",
					Protocol:    "icmp",
					ICMPType:    1,
					ICMPCode:    2,
					IPRanges: []store.IPRange{
						{
							Start: "2.2.3.4",
							End:   "2.2.3.5",
						},
					},
				},
				{
					Name:        "old-entry",
					Description: "this represents an entry that has no destination_metadata",
					Protocol:    "icmp",
					ICMPType:    1,
					ICMPCode:    2,
					IPRanges: []store.IPRange{
						{
							Start: "2.2.3.4",
							End:   "2.2.3.5",
						},
					},
				},
			}

			destinationStore := egressDestinationStore(db)
			createdEgressDestinations, err = destinationStore.Create(egressDestinations)
			Expect(err).ToNot(HaveOccurred())
			// delete one of the description_metadatas to simulate destinations that were created before the
			// destination_metadatas table existed
			_, err = db.Exec(`DELETE FROM destination_metadatas WHERE name='old-entry';`)
			Expect(err).ToNot(HaveOccurred())

			egressPolicies = []store.EgressPolicy{
				{
					Source: store.EgressSource{
						ID:   "some-app-guid",
						Type: "app",
					},
					Destination: store.EgressDestination{
						GUID: createdEgressDestinations[0].GUID,
					},
				},
				{
					Source: store.EgressSource{
						ID:   "space-guid",
						Type: "space",
					},
					Destination: store.EgressDestination{
						GUID: createdEgressDestinations[1].GUID,
					},
				},
				{
					Source: store.EgressSource{
						ID:   "different-app-guid",
						Type: "app",
					},
					Destination: store.EgressDestination{
						GUID: createdEgressDestinations[2].GUID,
					},
				},
				{
					Source: store.EgressSource{
						ID:   "different-space-guid",
						Type: "space",
					},
					Destination: store.EgressDestination{
						GUID: createdEgressDestinations[3].GUID,
					},
				},
			}

			createdEgressPolicies, err = egressStore.Create(egressPolicies)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("GetByGUID", func() {
			It("should return the requeted egress policies", func() {
				egressPolicies, err := egressPolicyTable.GetByGUID(tx, createdEgressPolicies[0].ID, createdEgressPolicies[1].ID)
				Expect(err).NotTo(HaveOccurred())
				Expect(egressPolicies).To(ConsistOf(
					store.EgressPolicy{
						ID: createdEgressPolicies[0].ID,
						Source: store.EgressSource{
							Type:         "app",
							TerminalGUID: createdEgressPolicies[0].Source.TerminalGUID,
							ID:           "some-app-guid",
						},
						Destination: createdEgressDestinations[0],
					},
					store.EgressPolicy{
						ID: createdEgressPolicies[1].ID,
						Source: store.EgressSource{
							Type:         "space",
							TerminalGUID: createdEgressPolicies[1].Source.TerminalGUID,
							ID:           "space-guid",
						},
						Destination: createdEgressDestinations[1],
					}))
			})

			Context("when a non-existent policy/no policy guid is requested", func() {
				It("returns an empty array", func() {
					egressPolicies, err := egressPolicyTable.GetByGUID(tx, "what-policy?")
					Expect(err).ToNot(HaveOccurred())
					Expect(egressPolicies).To(HaveLen(0))

					egressPolicies, err = egressPolicyTable.GetByGUID(tx)
					Expect(err).ToNot(HaveOccurred())
					Expect(egressPolicies).To(HaveLen(0))
				})
			})
		})

		Context("GetAllPolicies", func() {
			It("returns policies", func() {
				listedPolicies, err := egressPolicyTable.GetAllPolicies()
				Expect(err).ToNot(HaveOccurred())
				Expect(listedPolicies).To(HaveLen(4))
				Expect(listedPolicies).To(ConsistOf([]store.EgressPolicy{
					{
						ID: "guid-1",
						Source: store.EgressSource{
							ID:           "some-app-guid",
							Type:         "app",
							TerminalGUID: createdEgressPolicies[0].Source.TerminalGUID,
						},
						Destination: store.EgressDestination{
							GUID:        createdEgressDestinations[0].GUID,
							Name:        "a",
							Description: "desc a",
							Protocol:    "tcp",
							Ports: []store.Ports{
								{
									Start: 8080,
									End:   8081,
								},
							},
							IPRanges: []store.IPRange{
								{
									Start: "1.2.3.4",
									End:   "1.2.3.5",
								},
							},
						},
					},
					{
						ID: "guid-2",
						Source: store.EgressSource{
							ID:           "space-guid",
							Type:         "space",
							TerminalGUID: createdEgressPolicies[1].Source.TerminalGUID,
						},
						Destination: store.EgressDestination{
							GUID:        createdEgressDestinations[1].GUID,
							Name:        "b",
							Description: "desc b",
							Protocol:    "udp",
							IPRanges: []store.IPRange{
								{
									Start: "2.2.3.4",
									End:   "2.2.3.5",
								},
							},
						},
					},
					{
						ID: "guid-3",
						Source: store.EgressSource{
							ID:           "different-app-guid",
							Type:         "app",
							TerminalGUID: createdEgressPolicies[2].Source.TerminalGUID,
						},
						Destination: store.EgressDestination{
							GUID:        createdEgressDestinations[2].GUID,
							Name:        "c",
							Description: "desc c",
							Protocol:    "icmp",
							ICMPType:    1,
							ICMPCode:    2,
							IPRanges: []store.IPRange{
								{
									Start: "2.2.3.4",
									End:   "2.2.3.5",
								},
							},
						},
					},
					{
						ID: "guid-4",
						Source: store.EgressSource{
							ID:           "different-space-guid",
							Type:         "space",
							TerminalGUID: createdEgressPolicies[3].Source.TerminalGUID,
						},
						Destination: store.EgressDestination{
							GUID:        createdEgressDestinations[3].GUID,
							Name:        "",
							Description: "",
							Protocol:    "icmp",
							ICMPType:    1,
							ICMPCode:    2,
							IPRanges: []store.IPRange{
								{
									Start: "2.2.3.4",
									End:   "2.2.3.5",
								},
							},
						},
					},
				}))
			})

			Context("when the query fails", func() {
				It("returns an error", func() {
					setupEgressPolicyStore(mockDb)

					mockDb.QueryReturns(nil, errors.New("some error that sql would return"))

					egressPolicyTable = &store.EgressPolicyTable{
						Conn: mockDb,
					}

					_, err := egressPolicyTable.GetAllPolicies()
					Expect(err).To(MatchError("some error that sql would return"))
				})
			})
		})
	})

	Context("GetTerminalByAppGUID", func() {
		It("should return the terminal id for an app if it exists", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			terminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			_, err = egressPolicyTable.CreateApp(tx, terminalId, "some-app-guid")
			Expect(err).ToNot(HaveOccurred())

			foundID, err := egressPolicyTable.GetTerminalByAppGUID(tx, "some-app-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(foundID).To(Equal(terminalId))

			By("should return empty string and no error if the app is not found")
			foundID, err = egressPolicyTable.GetTerminalByAppGUID(tx, "garbage-app-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(foundID).To(Equal(""))
		})
	})

	Context("GetTerminalBySpaceGUID", func() {
		It("should return the terminal guid for a space if it exists", func() {
			db, tx := getMigratedRealDb(dbConf)
			setupEgressPolicyStore(db)

			terminalId, err := terminalsTable.Create(tx)
			Expect(err).ToNot(HaveOccurred())
			_, err = egressPolicyTable.CreateSpace(tx, terminalId, "some-space-guid")
			Expect(err).ToNot(HaveOccurred())

			foundID, err := egressPolicyTable.GetTerminalBySpaceGUID(tx, "some-space-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(foundID).To(Equal(terminalId))

			By("should return empty string and no error if the space is not found")
			foundID, err = egressPolicyTable.GetTerminalBySpaceGUID(tx, "garbage-space-guid")
			Expect(err).ToNot(HaveOccurred())
			Expect(foundID).To(Equal(""))
		})
	})

	Context("GetBySourceGuids", func() {
		Context("When using a real db", func() {
			var egressPolicies []store.EgressPolicy

			BeforeEach(func() {
				db, _ := getMigratedRealDb(dbConf)
				egressStore := setupEgressPolicyStore(db)

				egressDestinations := []store.EgressDestination{
					{
						Name:     "a",
						Protocol: "tcp",
						Ports: []store.Ports{
							{
								Start: 8080,
								End:   8081,
							},
						},
						IPRanges: []store.IPRange{
							{
								Start: "1.2.3.4",
								End:   "1.2.3.5",
							},
						},
					},
					{
						Name:     "b",
						Protocol: "udp",
						IPRanges: []store.IPRange{
							{
								Start: "2.2.3.4",
								End:   "2.2.3.5",
							},
						},
					},
					{
						Name:     "c",
						Protocol: "icmp",
						ICMPType: 1,
						ICMPCode: 2,
						IPRanges: []store.IPRange{
							{
								Start: "2.2.3.4",
								End:   "2.2.3.5",
							},
						},
					},
					{
						Name:     "d",
						Protocol: "udp",
						Ports: []store.Ports{
							{
								Start: 8080,
								End:   8081,
							},
						},
						IPRanges: []store.IPRange{
							{
								Start: "3.2.3.4",
								End:   "3.2.3.5",
							},
						},
					},
					{
						Name:     "e",
						Protocol: "udp",
						IPRanges: []store.IPRange{
							{
								Start: "2.2.3.4",
								End:   "2.2.3.5",
							},
						},
					},
				}

				createdDestinations, err := egressDestinationStore(db).Create(egressDestinations)
				Expect(err).ToNot(HaveOccurred())

				egressPolicies = []store.EgressPolicy{
					{
						Source: store.EgressSource{
							ID:   "some-app-guid",
							Type: "app",
						},
						Destination: store.EgressDestination{
							GUID: createdDestinations[0].GUID,
						},
					},
					{
						Source: store.EgressSource{
							ID:   "different-app-guid",
							Type: "app",
						},
						Destination: store.EgressDestination{
							GUID: createdDestinations[1].GUID,
						},
					},
					{
						Source: store.EgressSource{
							ID:   "different-app-guid",
							Type: "app",
						},
						Destination: store.EgressDestination{
							GUID: createdDestinations[2].GUID,
						},
					},
					{
						Source: store.EgressSource{
							ID:   "some-space-guid",
							Type: "space",
						},
						Destination: store.EgressDestination{
							GUID: createdDestinations[3].GUID,
						},
					},
					{
						Source: store.EgressSource{
							ID: "never-referenced-app-guid",
						},
						Destination: store.EgressDestination{
							GUID: createdDestinations[4].GUID,
						},
					},
				}
				_, err = egressStore.Create(egressPolicies)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when there are policies with the given id", func() {
				It("returns egress policies with those ids", func() {
					policies, err := egressPolicyTable.GetBySourceGuids([]string{"some-app-guid", "different-app-guid", "some-space-guid"})
					Expect(err).ToNot(HaveOccurred())
					Expect(policies).To(HaveLen(4))
					Expect(policies[0].Source.ID).To(Equal("some-app-guid"))
					Expect(policies[1].Source.ID).To(Equal("different-app-guid"))
					Expect(policies[2].Source.ID).To(Equal("different-app-guid"))
					Expect(policies[3].Source.ID).To(Equal("some-space-guid"))
				})
			})

			Context("when there are no policies with the given id", func() {
				It("returns no egress policies", func() {
					policies, err := egressPolicyTable.GetBySourceGuids([]string{"meow-this-is-a-bogus-app-guid"})
					Expect(err).ToNot(HaveOccurred())
					Expect(policies).To(HaveLen(0))
				})
			})
		})

		Context("when the query fails", func() {
			It("returns an error", func() {
				setupEgressPolicyStore(mockDb)

				mockDb.QueryReturns(nil, errors.New("some error that sql would return"))

				egressPolicyTable = &store.EgressPolicyTable{
					Conn: mockDb,
				}

				_, err := egressPolicyTable.GetBySourceGuids([]string{"id-does-not-matter"})
				Expect(err).To(MatchError("some error that sql would return"))
			})
		})
	})
})

func egressDestinationStore(db store.Database) *store.EgressDestinationStore {
	terminalsRepo := &store.TerminalsTable{
		Guids: &store.GuidGenerator{},
	}

	destinationMetadataTable := &store.DestinationMetadataTable{}
	egressDestinationStore := &store.EgressDestinationStore{
		Conn: db,
		EgressDestinationRepo:   &store.EgressDestinationTable{},
		TerminalsRepo:           terminalsRepo,
		DestinationMetadataRepo: destinationMetadataTable,
	}

	return egressDestinationStore
}
