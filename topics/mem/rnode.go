package mem

import (
	"github.com/troian/surgemq/message"
	topicsTypes "github.com/troian/surgemq/topics/types"
	"github.com/troian/surgemq/types"
)

// retained message nodes
type rNode struct {
	// If this is the end of the topic string, then add retained messages here
	msg *message.PublishMessage
	buf []byte

	// Otherwise add the next topic level here
	nodes map[string]*rNode
}

func newRNode() *rNode {
	return &rNode{
		nodes: make(map[string]*rNode),
	}
}

func (rn *rNode) insert(topic string, msg *message.PublishMessage) error {
	// If there's no more topic levels, that means we are at the matching rnode.
	if len(topic) == 0 {
		rn.msg = msg
		return nil
	}

	// Not the last level, so let's find or create the next level snode, and
	// recursively call it's insert().

	// ntl = next topic level
	ntl, rem, err := nextTopicLevel(topic)
	if err != nil {
		return err
	}

	level := ntl

	// Add sNode if it doesn't already exist
	n, ok := rn.nodes[level]
	if !ok {
		n = newRNode()
		rn.nodes[level] = n
	}

	return n.insert(rem, msg)
}

// Remove the retained message for the supplied topic
func (rn *rNode) remove(topic string) error {
	// If the topic is empty, it means we are at the final matching rnode. If so,
	// let's remove the buffer and message.
	if len(topic) == 0 {
		rn.buf = nil
		rn.msg = nil
		return nil
	}

	// Not the last level, so let's find the next level rnode, and recursively
	// call it's remove().

	// ntl = next topic level
	ntl, rem, err := nextTopicLevel(topic)
	if err != nil {
		return err
	}

	level := ntl

	// Find the rNode that matches the topic level
	n, ok := rn.nodes[level]
	if !ok {
		return types.ErrNotFound
	}

	// Remove the subscriber from the next level rnode
	if err := n.remove(rem); err != nil {
		return err
	}

	// If there are no more rNodes to the next level we just visited let's remove it
	if len(n.nodes) == 0 {
		delete(rn.nodes, level)
	}

	return nil
}

// match() finds the retained messages for the topic and qos provided. It's somewhat
// of a reverse match compare to match() since the supplied topic can contain
// wildcards, whereas the retained message topic is a full (no wildcard) topic.
func (rn *rNode) match(topic string, retained *[]*message.PublishMessage) error {
	// If the topic is empty, it means we are at the final matching rNode. If so,
	// add the retained msg to the list.
	if len(topic) == 0 {
		if rn.msg != nil {
			*retained = append(*retained, rn.msg)
		}
		return nil
	}

	// ntl = next topic level
	ntl, rem, err := nextTopicLevel(topic)
	if err != nil {
		return err
	}

	level := ntl

	if level == topicsTypes.MWC {
		// If '#', add all retained messages starting this node
		rn.allRetained(retained)
	} else if level == topicsTypes.SWC {
		// If '+', check all nodes at this level. Next levels must be matched.
		for _, n := range rn.nodes {
			if err := n.match(rem, retained); err != nil {
				return err
			}
		}
	} else {
		// Otherwise, find the matching node, go to the next level
		if n, ok := rn.nodes[level]; ok {
			if err := n.match(rem, retained); err != nil {
				return err
			}
		}
	}

	return nil
}

func (rn *rNode) allRetained(retained *[]*message.PublishMessage) {
	if rn.msg != nil {
		*retained = append(*retained, rn.msg)
	}

	for _, n := range rn.nodes {
		n.allRetained(retained)
	}
}
