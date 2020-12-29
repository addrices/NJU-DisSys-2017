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

// import "bytes"
// import "encoding/gob"
import "fmt"
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
	HeartBeat bool		//is HeartBeat
	Index int			//index
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

	rf.currentTerm += 1
	rf.votedFor = rf.me
	var votesCount int = 1
	fmt.Printf("%d ask Vote!! term:%d total:%d\n",rf.me,rf.currentTerm,len(rf.peers))

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
					if(votesCount > (len(rf.peers))/2){
						fmt.Printf("%d is new LEADER\n",rf.me)
						for i := 0;i < len(rf.logs);i++{
							fmt.Printf("%d %d\n",rf.logs[i].Command,rf.logs[i].Term)
						}					
						rf.state = "LEADER"
						for i := 0; i < len(rf.peers); i++ {
							if i == rf.me {
								continue
							}
							rf.nextIndex[i] = len(rf.logs)
							rf.matchIndex[i] = -1
						}
						rf.SendAppendEntries()
						rf.restartTime()
					}
				}
			}
		}(peer, args)
	}
	rf.restartTime()
}

func (rf *Raft) persist() {
	// Your code here.
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here.
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
}

func (rf *Raft) commit() {
	// var args ApplyMsg
	// if (rf.state == "LEADER") == true{
	// 	fmt.Printf("isLeader\n")
	// } else {
	// 	fmt.Printf("isNotLeader\n")
	// }
	// fmt.Printf("rf.me:%d\n",rf.me)
	// fmt.Printf("index:%d\n",rf.lastApplied)
	// fmt.Printf("rf.logs:%d\n",rf.logs[rf.lastApplied].Command)

	// rf.mu.Lock()
	// defer rf.mu.Unlock()
	var args ApplyMsg
	args.Index = rf.lastApplied+1
	args.Command = rf.logs[rf.lastApplied].Command
	fmt.Printf("%d commit %d %d\n",rf.me,rf.lastApplied,args.Command)
	rf.applyCh <- args
}

func (rf *Raft) AppendEntries(args AppendEntryArg,reply *AppendEntryReply){
	rf.mu.Lock()
	defer rf.mu.Unlock()
	// fmt.Printf("%d receive heartbeat\n",rf.me)
	reply.HeartBeat = false
	reply.Index = args.PrevLogIndex+1
	if args.Term < rf.currentTerm{
		reply.Success = false
		reply.Term = rf.currentTerm
		fmt.Printf("AppendEntries:args.Term < rf.currentTerm voterequest from %d \n",args.LeaderId)
	} else {
		rf.state = "FOLLOWER"
		rf.currentTerm = args.Term
		reply.Term = args.Term
		rf.votedFor = -1
		if args.Entries != nil{
			fmt.Printf("%d receive AppendEntries command:%d Term:%d PrevLogIndex:%d PrevLogTerm:%d LeaderCommit:%d",rf.me,args.Entries[0].Command,args.Term,args.PrevLogIndex,args.PrevLogTerm,args.LeaderCommit)
		} 

		if args.PrevLogIndex >=0 && (len(rf.logs)-1 < args.PrevLogIndex || rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm){
			reply.Success = false
			rf.logs = rf.logs[:args.PrevLogIndex]
		} else if args.Entries != nil{	//说明发送的带有entry
			fmt.Printf("%d get log %d len(rf):%d\n",rf.me, args.PrevLogIndex,len(rf.logs))
			fmt.Printf("%d receive Leadercommit %d localcommit %d \n",rf.me, args.LeaderCommit,rf.commitIndex)
			// fmt.Printf("===========%d logs==========\n",rf.me)
			// fmt.Printf("args.PrevLogIndex:%d\n",args.PrevLogIndex)
			// for i := 0; i < len(rf.logs); i++ {
			// 	fmt.Printf("%d command:%d term:%d\n",i,rf.logs[i].Command,rf.logs[i].Term)
			// }
			fmt.Printf("before args.PrevLogIndex:%d\n",args.PrevLogIndex)
			for i:= 0;i < len(rf.logs);i++{
				fmt.Printf("%d command:%d term:%d\n",i,rf.logs[0].Command,rf.logs[0].Term)
			}
			rf.logs = rf.logs[:args.PrevLogIndex+1]
			fmt.Printf("middle\n")
			for i:= 0;i < len(rf.logs);i++{
				fmt.Printf("%d command:%d term:%d\n",i,rf.logs[0].Command,rf.logs[0].Term)
			}
			rf.logs = append(rf.logs, args.Entries ...)
			fmt.Printf("after\n")
			for i:= 0;i < len(rf.logs);i++{
				fmt.Printf("%d command:%d term:%d\n",i,rf.logs[0].Command,rf.logs[0].Term)
			}
			reply.Success = true
			if rf.commitIndex < args.LeaderCommit{
				rf.commitIndex = args.LeaderCommit
				if(args.LeaderCommit > len(rf.logs)-1){
					rf.commitIndex = len(rf.logs)-1
				}
				rf.lastApplied = rf.commitIndex
				go rf.commit()
			}
		} else{							//纯心跳
			fmt.Printf("%d receive heartbeat commitIndex %d LeaderCommit %d\n",rf.me,rf.commitIndex,args.LeaderCommit)
			if rf.commitIndex < args.LeaderCommit{
				rf.commitIndex = args.LeaderCommit
				if(args.LeaderCommit > len(rf.logs)-1){
					rf.commitIndex = len(rf.logs)-1
				}
				rf.lastApplied = rf.commitIndex
				go rf.commit()
			}

			reply.Success = true
			reply.HeartBeat = true
		}
		// AppendEntryReply		
		// fmt.Printf("%d get heartbeat, restart \n",rf.me)
		rf.restartTime()
	}
}

func (rf *Raft) SendAppendEntry(peer int){
	if peer == rf.me {
		return
	}
	var entry AppendEntryArg
	entry.LeaderId = rf.me
	entry.LeaderCommit = rf.commitIndex
	entry.Term = rf.currentTerm


	entry.PrevLogIndex = rf.nextIndex[peer]-1
	if entry.PrevLogIndex >=0 {
		entry.PrevLogTerm = rf.logs[entry.PrevLogIndex].Term
	}
	if rf.nextIndex[peer] < len(rf.logs){
		entry.Entries = rf.logs[rf.nextIndex[peer]:]
		fmt.Printf("SEND ENTRIES rf.nextIndex[peer]:%d\n",rf.nextIndex[peer])
		for i:= 0;i < len(entry.Entries);i++{
			fmt.Printf("%d command:%d term:%d\n",i,rf.logs[i].Command,rf.logs[i].Term)
		}
		// fmt.Printf("%d len:%d rf.nextIndex[peer]:%d\n",rf.me,len(rf.logs),rf.nextIndex[peer])
	} else {
		entry.Entries = nil
		// fmt.Printf("%d len:%d rf.nextIndex[peer]:%d\n",rf.me,len(rf.logs),rf.nextIndex[peer])
	}

	go func(server int, args AppendEntryArg) {
		var reply AppendEntryReply
		if args.Entries != nil{
			fmt.Printf("%d send AppendEntries peer:%d Term:%d LeaderId:%d PrevLogIndex:%d LeaderCommit:%d\n",rf.me,peer,entry.Term, entry.LeaderId, entry.PrevLogIndex, entry.LeaderCommit)
		}
		ok := rf.peers[server].Call("Raft.AppendEntries", args, &reply)
		if ok {
			rf.handleAppendEntry(server, reply)
		}
	}(peer, entry)
}

func (rf *Raft) handleAppendEntry(server int, reply AppendEntryReply) {
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

	//接受了 
	if reply.Success && reply.HeartBeat == false{
		rf.nextIndex[server] = reply.Index + 1
		rf.matchIndex[server] = reply.Index
		// fmt.Printf("handle %d rf.matchIndex %d %d \n",reply.Index,server,rf.matchIndex[server])
		count :=1
		for i := 0; i < len(rf.peers); i = i+1{
			if i!=rf.me && rf.matchIndex[i] >= rf.matchIndex[server]{
				count += 1
			}
			//fmt.Printf("\nhandleAppendEntries: "+strconv.Itoa(i))
		}	
		// fmt.Printf("count %d %d \n",count,len(rf.peers)/2)	
		if count > len(rf.peers)/2{
			rf.commitIndex = reply.Index
			fmt.Printf("Leader commit update index %d\n",rf.commitIndex)
			rf.lastApplied = rf.commitIndex
			go rf.commit()
		}
	}else if reply.HeartBeat == false{
		if rf.nextIndex[server] >= 0{
			rf.nextIndex[server] = rf.nextIndex[server] - 1
		}
		rf.SendAppendEntry(server)
	}
}

func (rf *Raft) SendAppendEntries() {
	// fmt.Printf("%d sendAppendEntry\n",rf.me)
	for peer := 0; peer < len(rf.peers); peer++{
		rf.SendAppendEntry(peer)
	}
}

func (rf *Raft) Timeout() {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != "LEADER" {
		rf.state = "CANDIDATE"
		fmt.Printf("peer %d Timeout,term:%d,i'm askvote\n",rf.me,rf.currentTerm)
		rf.AskVote()
	} else {
		rf.SendAppendEntries()
	}
	rf.restartTime()
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
		if can_vote == false{
			fmt.Printf("%d i'cant vote CandidateId:%d it's LastLogTerm:%d myLastLogTerm:%d\n",rf.me,args.CandidateId,args.LastLogTerm,rf.logs[len(rf.logs)-1].Term)
		}
	}

	if args.Term == rf.currentTerm {
		if rf.votedFor == -1 && can_vote{
			if can_vote{
				rf.votedFor = args.CandidateId
			}
			reply.VoteGranted = can_vote
		} else {
			reply.Term = rf.currentTerm
			reply.VoteGranted = (rf.votedFor == args.CandidateId)
		}
		// fmt.Printf("i'm %d args.Term %d rf.currentTerm %d, i vote %d\n",rf.me,args.Term,rf.currentTerm,args.CandidateId)
		if reply.VoteGranted {
			fmt.Printf("%d vote %d",rf.me,rf.votedFor)
			for i := 0;i < len(rf.logs);i++{
				fmt.Printf("%d %d\n",rf.logs[i].Command,rf.logs[i].Term)
			}	
			rf.restartTime()
		}
		return
	}

	if args.Term > rf.currentTerm {

		rf.state = "FOLLOWER"
		// fmt.Printf("currentTerm %d, Term %d\n",rf.currentTerm, args.Term)
		rf.currentTerm = args.Term
		rf.votedFor = -1

		if(can_vote){
			rf.votedFor = args.CandidateId
		}

		// fmt.Printf("i'm %d args.Term %d rf.currentTerm %d, i vote %d and i restart\n",rf.me,args.Term,rf.currentTerm,rf.votedFor)

		reply.Term = args.Term
		reply.VoteGranted = (rf.votedFor == args.CandidateId)
		// fmt.Printf("\nreceive request: "+strconv.Itoa(rf.me)+" "+ strconv.Itoa(len(rf.logs))+" "+ strconv.Itoa(rf.currentTerm))
		if reply.VoteGranted {
			fmt.Printf("%d vote %d\n",rf.me,rf.votedFor)
			for i := 0;i < len(rf.logs);i++{
				fmt.Printf("%d %d\n",rf.logs[i].Command,rf.logs[i].Term)
			}	
			rf.restartTime()
		}

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
	term := rf.currentTerm
	isLeader := false
	if rf.state != "LEADER"{
		return index, term, isLeader
	}

	var newlog LogEntry
	newlog.Command = command
	newlog.Term = rf.currentTerm
	rf.logs = append(rf.logs,newlog)
	index = len(rf.logs)
	// for i := 0;i < len(rf.nextIndex);i++{
	// 	fmt.Printf("%d nextIndex %d\n",i,rf.nextIndex[i])
	// }
	fmt.Printf("rf start command %d \n",newlog.Command)
	fmt.Printf("%d len(rflog):%d\n",rf.me,len(rf.logs))
	rf.SendAppendEntries()
	isLeader = true
	term = rf.currentTerm
	// fmt.Printf("return\n")

	return index, term, isLeader
}

func (rf *Raft) GetRfLenLog() (int){
	return len(rf.logs)
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
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here.
	rf.currentTerm = 0
	rf.votedFor = -1
	rf.state = "FOLLOWER"
	rf.applyCh = applyCh

	rf.logs = make([]LogEntry,0)
	rf.commitIndex = -1
	rf.lastApplied = -1

	rf.nextIndex = make([]int,len(peers))
	rf.matchIndex = make([]int,len(peers))
	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())
	rf.persist()
	rf.restartTime()

	return rf
}
