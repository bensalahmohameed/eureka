// frameworkImpl is the component responsible for initializing and running scheduler plugins.
type frameworkImpl struct {
	registry             Registry
	snapshotSharedLister framework.SharedLister
	waitingPods          *waitingPodsMap
	scorePluginWeight    map[string]int
	preEnqueuePlugins    []framework.PreEnqueuePlugin
	enqueueExtensions    []framework.EnqueueExtensions
	queueSortPlugins     []framework.QueueSortPlugin
	preFilterPlugins     []framework.PreFilterPlugin
	filterPlugins        []framework.FilterPlugin
	postFilterPlugins    []framework.PostFilterPlugin
	preScorePlugins      []framework.PreScorePlugin
	scorePlugins         []framework.ScorePlugin
	reservePlugins       []framework.ReservePlugin
	preBindPlugins       []framework.PreBindPlugin
	bindPlugins          []framework.BindPlugin
	postBindPlugins      []framework.PostBindPlugin
	permitPlugins        []framework.PermitPlugin

	// pluginsMap contains all plugins, by name.
	pluginsMap map[string]framework.Plugin

	clientSet       clientset.Interface
	kubeConfig      *restclient.Config
	eventRecorder   events.EventRecorder
	informerFactory informers.SharedInformerFactory
	logger          klog.Logger

	metricsRecorder          *metrics.MetricAsyncRecorder
	profileName              string
	percentageOfNodesToScore *int32

	extenders []framework.Extender
	framework.PodNominator

	parallelizer parallelize.Parallelizer
}


	pluginsMap map[string]framework.Plugin
func (f *frameworkImpl) runScorePlugin(ctx context.Context, pl framework.ScorePlugin, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	s, status := pl.Score(ctx, state, pod, nodeName)
	return s, status
}

// RunScorePlugins runs the set of configured scoring plugins.
// It returns a list that stores scores from each plugin and total score for each Node.
// It also returns *Status, which is set to non-success if any of the plugins returns
// a non-success status.
func (f *frameworkImpl) RunScorePlugins(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodes []*framework.NodeInfo) (ns []framework.NodePluginScores, status *framework.Status) {
	
	// Run Score method for each node in parallel.
	if len(plugins) > 0 {
	f.Parallelizer().Until(ctx, len(nodes), func(index int) {
		nodeName := nodes[index].Node().Name
		for _, pl := range plugins {
			s, status := f.runScorePlugin(ctx, pl, state, pod, nodeName)
			if !status.IsSuccess() {
				err := fmt.Errorf("plugin %q failed with: %w", pl.Name(), status.AsError())
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			pluginToNodeScores[pl.Name()][index] = framework.NodeScore{
				Name:  nodeName,
				Score: s,
			}
		}
	}, metrics.Score)
}

	// Run NormalizeScore method for each ScorePlugin in parallel.
	f.Parallelizer().Until(ctx, len(plugins), func(index int) {
		pl := plugins[index]
		nodeScoreList := pluginToNodeScores[pl.Name()]
		status := f.runScoreExtension(ctx, pl, state, pod, nodeScoreList)
	}, metrics.Score)

	// Apply score weight for each ScorePlugin in parallel,
	// and then, build allNodePluginScores.
	f.Parallelizer().Until(ctx, len(nodes), func(index int) {
		nodePluginScores := framework.NodePluginScores{
			Name:   nodes[index].Node().Name,
			Scores: make([]framework.PluginScore, len(plugins)),
		}

		for i, pl := range plugins {
			weight := f.scorePluginWeight[pl.Name()]
			nodeScoreList := pluginToNodeScores[pl.Name()]
			score := nodeScoreList[index].Score

			if score > framework.MaxNodeScore || score < framework.MinNodeScore {
				err := fmt.Errorf("plugin %q returns an invalid score %v, it should in the range of [%v, %v] after normalizing", pl.Name(), score, framework.MinNodeScore, framework.MaxNodeScore)
				errCh.SendErrorWithCancel(err, cancel)
				return
			}
			weightedScore := score * int64(weight)
			nodePluginScores.Scores[i] = framework.PluginScore{
				Name:  pl.Name(),
				Score: weightedScore,
			}
			nodePluginScores.TotalScore += weightedScore
		}
		allNodePluginScores[index] = nodePluginScores
	}, metrics.Score)


	return allNodePluginScores, nil
}
