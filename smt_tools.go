/**
 *  @file
 *  @copyright defined in aergo/LICENSE.txt
 */

package trie

// The Package Trie implements a sparse merkle trie.

import (
	"bytes"
	"fmt"
)

// Get fetches the value of a key by going down the current trie root.
func (s *SMT) Get(key []byte) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	s.atomicUpdate = false
	return s.get(s.Root, key, nil, 0, s.TrieHeight)
}

// get fetches the value of a key given a trie root
func (s *SMT) get(root []byte, key []byte, batch [][]byte, iBatch uint8, height uint64) ([]byte, error) {
	if len(root) == 0 {
		return nil, nil
	}
	if height == 0 {
		return root[:HashLength], nil
	}
	// Fetch the children of the node
	batch, iBatch, lnode, rnode, isShortcut, err := s.loadChildren(root, height, batch, iBatch)
	if err != nil {
		return nil, err
	}
	if isShortcut {
		if bytes.Equal(lnode[:HashLength], key) {
			return rnode[:HashLength], nil
		}
		return nil, nil
	}
	if bitIsSet(key, s.TrieHeight-height) {
		return s.get(rnode, key, batch, 2*iBatch+2, height-1)
	}
	return s.get(lnode, key, batch, 2*iBatch+1, height-1)
}

// DefaultHash is a getter for the defaultHashes array
func (s *SMT) DefaultHash(height uint64) []byte {
	return s.defaultHashes[height]
}

// CheckRoot returns true if the root exists in Database.
func (s *SMT) CheckRoot(root []byte) bool {
	s.db.lock.RLock()
	dbval := s.db.store.Get(root)
	s.db.lock.RUnlock()
	if len(dbval) != 0 {
		return true
	}
	return false
}

// Commit stores the updated nodes to disk
// Commit should be called for every block otherwise past tries are not recorded and it is not possible to revert to them
func (s *SMT) Commit() error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if s.db.store == nil {
		return fmt.Errorf("DB not connected to trie")
	}
	// Commit the new nodes to database, clear updatedNodes and store the Root in history for reverts.
	if !s.atomicUpdate {
		if len(s.pastTries) >= maxPastTries {
			copy(s.pastTries, s.pastTries[1:])
			s.pastTries[len(s.pastTries)-1] = s.Root
		} else {
			s.pastTries = append(s.pastTries, s.Root)
		}
	}
	s.db.commit()
	s.db.updatedNodes = make(map[Hash][][]byte)
	return nil
}

// RollbackTo rolls back the changes made by previous updates
// and loads the cache from before the rollback.
func (s *SMT) Stash() {
	// Making a temporary liveCache requires it to be copied, so it's quicker
	// to just load the cache if a block state root was incorrect.
	s.Root = s.prevRoot
	s.db.liveCache = make(map[Hash][][]byte)
	s.db.updatedNodes = make(map[Hash][][]byte)
}
