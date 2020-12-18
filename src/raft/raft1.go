package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, Term, isleader)
//   start agreement on a new log entry
// rf.GetState() (Term, isLeader)
//   ask a Raft for its current Term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import "sync"
import "labrpc"

import "bytes"
import "encoding/gob"
// import "fmt"
import "time"
import "math/rand"


//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type LogEntry struct{
	Command interface{}
	Term int				//index can get from Log[]array
}

const (
	HeartbeatTime  = 100
	ElectionMinTime = 200
	ElectionMaxTime = 400
)


//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex
	peers     []*labrpc.ClientEnd
	persister *Persister
	me        int // index into peers[]
	
	// Your data here.
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	currentTerm int 	// latest Term server has seen
	votedFor int 		//candidateId that received vote in current Term
	logs []LogEntry 	//log entries;each entry contains command for state machine
	
	commitIndex int 	//index of highest log entry known to be committed
	lastApplied int		//index of highest log entry applied to state machine
	
	nextIndex []int		//for each server,index of the next log entry to send to that server
	matchIndex []int 	//for each server,index opf highest log entry known to be replicated on server
	
	votedMe int 		//how many node vote me
	state string		//follower candidate leader
	applyCh chan ApplyMsg	
	
	timer *time.Timer

}

//AppendEntries RPC
type AppendEntryArg struct{
	Term int	 		//leader's Term
	LeaderId int 		//so follower can redirect clients
	PrevLogIndex int 	//index of log entry immediately preceding new ones
	PrevLogTerm int 	//Term of prevLogIndex entry
	Entries []LogEntry	//log entries to store
	LeaderCommit int	//leader's commitIndex
}

type AppendEntryReply struct{
	Term int			//currentTerm,for leader to update itself
	Success bool		//true if follower contained entry matching prevLogIndex and prevLogTerm
	CommitIndex int
}


// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	
	var Term int
	var isleader bool
	// Your code here.
	Term = rf.currentTerm
	isleader = (rf.state == "LEADER")
	return Term, isleader
}

func (rf *Raft) GetState_me() (term *int,isleader *bool) {
	
	// Your code here.
	*term = rf.currentTerm
	*isleader = (rf.state == "LEADER")
	return
}


//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here.
	// Example:
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.logs)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	d.Decode(&rf.currentTerm)
	d.Decode(&rf.logs)
}




//
// example RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	// Your data here.
	Term         int // candidate's Term
	CandidateId  int // candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // Term of candidate's last log entry
}

//
// example RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	// Your data here.
	Term        int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

func (rf *Raft) AskVote() {
	// for peer := 0; peer < len(rf.peers); peer++ {
	// 	if peer == rf.me {
	// 		continue
	// 	}
	// 	go func(peer int) {
	// 		var term int
	// 		var isLeader bool
	// 		if rf.peers[peer] 
	// 		ok := rf.peers[peer].Call("Raft.GetState_me", &term, &isLeader)
	// 		if ok {
	// 			if isLeader{
	// 				fmt.Printf("%d is leader, term = %d\n",peer,term)
	// 			} else{
	// 				fmt.Printf("%d is not leader, term = %d\n",peer,term)
	// 			}
	// 		}
	// 	}(peer)

	// }
	rf.currentTerm += 1
	rf.votedFor = rf.me
	var votesCount int = 1
	rf.persist()
	// fmt.Printf("%d ask Vote!! term:%d state:%s\n",rf.me,rf.currentTerm,rf.state)

	var args RequestVoteArgs
	args.Term = rf.currentTerm
	args.CandidateId = rf.me
	args.LastLogIndex = len(rf.logs) -1

	if args.LastLogIndex>=0{
		args.LastLogTerm = rf.logs[args.LastLogIndex].Term
	}

	for peer := 0; peer < len(rf.peers); peer++ {
		// jump myself
		if peer == rf.me {
			continue
		}
		//ask other node vote me
		go func(peer int, args RequestVoteArgs) {
			var reply RequestVoteReply
			// fmt.Printf("RequestVote %d\n",peer)
			ok := rf.peers[peer].Call("Raft.RequestVote", args, &reply)
			if ok {
				if reply.VoteGranted == true{
					votesCount++;
					if(votesCount >= (len(rf.peers))/2 + 1){
						// fmt.Printf("%d is new LEADER\n",rf.me)						
						rf.state = "LEADER"
						for i := 0; i < len(rf.peers); i++ {
							if i == rf.me {
								continue
							}
							rf.nextIndex[i] = len(rf.logs)
							rf.matchIndex[i] = -1
						}
					}
				}
			}
		}(peer, args)

	}
}

func (rf *Raft) AppendEntries(args AppendEntryArg,reply *AppendEntryReply){
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// fmt.Printf("%d receive heartbeat",rf.me)
	if args.Term < rf.currentTerm{
		reply.Success = false
		reply.Term = rf.currentTerm
		// fmt.Printf("AppendEntries:args.Term < rf.currentTerm voterequest from %d \n",args.leader)
	} else {
		// rf.state = "FOLLOWER"
		// rf.currentTerm = args.Term
		// rf.votedFor = reply.Term
		
		// reply.Success = true
		// AppendEntryReply		
		// reply.Term = rf.currentTerm
		rf.state = "FOLLOWER"
		// fmt.Printf("%d get heartbeat, restart \n",rf.me)
		rf.restartTime()
	}
}

func (rf *Raft) handleAppendEntries(server int, reply AppendEntryReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != "LEADER" {
		return
	}

	// "LEADER" should degenerate to Follower
	if reply.Term > rf.currentTerm {
		rf.currentTerm = reply.Term
		// fmt.Printf("%d degenerate to Follower\n",rf.me)
		rf.state = "FOLLOWER"
		rf.votedFor = -1
		//rf.log
		rf.restartTime()
		return
	}

	// if reply.Success{
	// 	rf.nextIndex[server] = reply.CommitIndex + 1
	// 	rf.matchIndex[server] = reply.CommitIndex
	// 	count :=1
	// 	i :=0
	// 	for i < len(rf.peers){
	// 		if i!=rf.me && rf.matchIndex[i] >= rf.matchIndex[server]{
	// 			count += 1
	// 		}
	// 		i++
	// 	}
	// 	if count >= len(rf.peers)/2+1{
	// 		if rf.commitIndex < rf.matchIndex[server] &&
	// 		rf.logs[rf.matchIndex[server]].Term == rf.currentTerm {
	// 			rf.commitIndex = rf.matchIndex[server]
	// 			go rf.commit()
	// 		}
	// 	}

	// }else{
	// 	rf.nextIndex[server] = reply.CommitIndex + 1
	// 	rf.SendAppendEntries()
	// }
}


func (rf *Raft) SendAppendEntries() {


	// fmt.Printf("%d sendAppendEntry\n",rf.me)
	for peer := 0; peer < len(rf.peers); peer++{
		if peer == rf.me {
			continue
		}
		var entry AppendEntryArg
		entry.Term = rf.currentTerm
		entry.LeaderId = rf.me
		//fmt.Printf("\nSendAppendEntries:1234 "+strconv.Itoa(rf.me))
		entry.PrevLogIndex = rf.nextIndex[peer]-1
		//fmt.Printf("\nSendAppendEntries:1234 "+strconv.Itoa(rf.me))
		if entry.PrevLogIndex >=0 {
			//fmt.Printf("\nSendAppendEntries:12345 "+strconv.Itoa(args.PrevLogIndex)+" LEN: "+strconv.Itoa(len(rf.logs)))
			entry.PrevLogTerm = rf.logs[entry.PrevLogIndex].Term
			//fmt.Printf("\nSendAppendEntries:12346 "+strconv.Itoa(rf.me))
		}
		//fmt.Printf("\nSendAppendEntries:1234 "+strconv.Itoa(rf.me))
		if rf.nextIndex[peer] < len(rf.logs) {
			entry.Entries = rf.logs[rf.nextIndex[peer]:]
		}

		entry.LeaderCommit = rf.commitIndex
		go func(server int, args AppendEntryArg) {
			var reply AppendEntryReply
			ok := rf.peers[server].Call("Raft.AppendEntries", args, &reply)
			if ok {
				// rf.handleAppendEntries(server, reply)
				// fmt.Printf("appendEntry go")
				// rf.handleAppendEntries(server, reply)
			}
		}(peer, entry)
	}
}


func (rf *Raft) Timeout() {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	// fmt.Printf("peer %d Timeout,i'm %s\n",rf.me,rf.state)
	if rf.state != "LEADER" {
		rf.state = "CANDIDATE"
		rf.AskVote()
	} else {
		// it's time to send heartbeats
		// fmt.Printf("go heart\n")
		// fmt.Printf("%d is leader\n",rf.me)
		rf.SendAppendEntries()
		//fmt.Printf("\nSendAppendEntries: "+strconv.Itoa(rf.me)+" "+strconv.Itoa(rf.currentTerm)+" "+rf.state)
	}
	rf.restartTime()
	//fmt.Printf("\nrestart time in Timeout: "+strconv.Itoa(rf.me))
}

//Raft 开始计时
func (rf *Raft) restartTime() {

	randst := ElectionMinTime+rand.Int63n(ElectionMaxTime-ElectionMinTime)
	timeout := time.Millisecond * time.Duration(randst)
	if rf.state == "LEADER" {
		timeout = time.Millisecond * HeartbeatTime
		randst = HeartbeatTime
	}
	if rf.timer == nil {
		rf.timer = time.NewTimer(timeout)
		go func() {
			for {
				//will send sign when timeout
				<-rf.timer.C
				//cite Timeout
				rf.Timeout()
			}
		}()
	}
	rf.timer.Reset(timeout)
	//fmt.Printf("\nrestart time: "+strconv.Itoa(rf.me)+"  time: "+strconv.Itoa(int(randst)))
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here.
	rf.mu.Lock()
	defer rf.mu.Unlock()
	
	// fmt.Printf("i'm %d args.Term %d rf.currentTerm %d, %d want me vote\n",rf.me,args.Term,rf.currentTerm,args.CandidateId)

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	can_vote := true
	if len(rf.logs)>0{
		if rf.logs[len(rf.logs)-1].Term > args.LastLogTerm {
			can_vote = false
		} else if rf.logs[len(rf.logs)-1].Term == args.LastLogTerm && len(rf.logs)-1 > args.LastLogIndex {
			can_vote = false
		}
	}

	if args.Term == rf.currentTerm {
		if rf.votedFor == -1 && can_vote{
			if can_vote{
				rf.votedFor = args.CandidateId
				rf.persist()
			}
			reply.VoteGranted = can_vote
		} else {
			reply.Term = rf.currentTerm
			reply.VoteGranted = (rf.votedFor == args.CandidateId)
		}
		// fmt.Printf("i'm %d args.Term %d rf.currentTerm %d, i vote %d\n",rf.me,args.Term,rf.currentTerm,args.CandidateId)
		rf.restartTime()
		return
	}

	if args.Term > rf.currentTerm {

		rf.state = "FOLLOWER"
		// fmt.Printf("currentTerm %d, Term %d\n",rf.currentTerm, args.Term)
		rf.currentTerm = args.Term
		rf.votedFor = -1

		if(can_vote){
			rf.votedFor = args.CandidateId
			rf.persist()
		}

		rf.restartTime()
		// fmt.Printf("i'm %d args.Term %d rf.currentTerm %d, i vote %d and i restart\n",rf.me,args.Term,rf.currentTerm,rf.votedFor)

		reply.Term = args.Term
		reply.VoteGranted = (rf.votedFor == args.CandidateId)
		// fmt.Printf("\nreceive request: "+strconv.Itoa(rf.me)+" "+ strconv.Itoa(len(rf.logs))+" "+ strconv.Itoa(rf.currentTerm))

		return
	}

	
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// returns true if labrpc says the RPC was delivered.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}


//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// Term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := -1
	isLeader := false
	if rf.state != "LEADER"{
		return index, term, isLeader
	}

	var newlog LogEntry
	// new Log come
	newlog.Command = command
	newlog.Term = rf.currentTerm
	rf.logs = append(rf.logs,newlog)
	// index come rf.logs
	index = len(rf.logs)
	isLeader = true
	term = rf.currentTerm
	// save some data
	rf.persist()


	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.applyCh = applyCh	//what's this

	// Your initialization code here.
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.state = "FOLLOWER"
	rf.logs = make([]LogEntry,0)
	rf.commitIndex = -1
	rf.lastApplied = -1

	rf.nextIndex = make([]int,len(peers))
	rf.matchIndex = make([]int,len(peers))
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.persist()
	// fmt.Println("Hello")
	rf.restartTime()

	return rf
}
