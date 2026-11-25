package workspace

// QueryExecution 跟踪当前唯一活动查询，使用递增序号隔离过期异步事件。
type QueryExecution struct {
	generation uint64
	tab        TabID
	running    bool
}

// Start 开始指定标签页的查询，并使此前所有查询事件失效。
func (q *QueryExecution) Start(tab TabID) uint64 {
	if q == nil { return 0 }
	q.generation++
	q.tab = tab
	q.running = tab != 0
	return q.generation
}

// IsCurrent 判断事件是否仍属于当前活动查询。
func (q *QueryExecution) IsCurrent(generation uint64, tab TabID) bool {
	return q != nil && q.running && generation == q.generation && tab == q.tab
}

// Finish 结束当前查询；过期查询不能结束新查询。
func (q *QueryExecution) Finish(generation uint64, tab TabID) bool {
	if !q.IsCurrent(generation, tab) { return false }
	q.running = false
	return true
}

// Cancel 使当前查询及其尚未消费的事件全部失效。
func (q *QueryExecution) Cancel() {
	if q == nil { return }
	q.generation++
	q.tab = 0
	q.running = false
}
