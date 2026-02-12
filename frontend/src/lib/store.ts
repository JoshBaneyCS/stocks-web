import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, MarketStatus, InstrumentListItem } from './types';
import * as api from './api';

// ─── Auth Store ───

interface AuthState {
  user: User | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (
    email: string,
    password: string,
    firstName: string,
    lastName: string,
    referralCode?: string
  ) => Promise<void>;
  logout: () => Promise<void>;
  checkAuth: () => Promise<void>;
  setToken: (token: string) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: true,

      login: async (email: string, password: string) => {
        const res = await api.login(email, password);
        set({
          user: res.user,
          accessToken: res.access_token,
          isAuthenticated: true,
        });
      },

      signup: async (
        email: string,
        password: string,
        firstName: string,
        lastName: string,
        referralCode?: string
      ) => {
        const res = await api.signup(email, password, firstName, lastName, referralCode);
        set({
          user: res.user,
          accessToken: res.access_token,
          isAuthenticated: true,
        });
      },

      logout: async () => {
        try {
          await api.logout();
        } catch {
          // ignore logout errors
        }
        set({
          user: null,
          accessToken: null,
          isAuthenticated: false,
        });
      },

      checkAuth: async () => {
        const { accessToken } = get();
        if (!accessToken) {
          set({ isLoading: false, isAuthenticated: false });
          return;
        }
        try {
          const user = await api.getMe();
          set({ user, isAuthenticated: true, isLoading: false });
        } catch {
          set({
            user: null,
            accessToken: null,
            isAuthenticated: false,
            isLoading: false,
          });
        }
      },

      setToken: (token: string) => {
        set({ accessToken: token });
      },
    }),
    {
      name: 'stocks-auth',
      partialize: (state) => ({
        accessToken: state.accessToken,
      }),
    }
  )
);

// ─── Market Store ───

interface MarketState {
  status: MarketStatus | null;
  isOpen: boolean;
  isLoading: boolean;
  error: string | null;
  fetchStatus: () => Promise<void>;
  startPolling: () => () => void;
}

export const useMarketStore = create<MarketState>()((set) => ({
  status: null,
  isOpen: false,
  isLoading: false,
  error: null,

  fetchStatus: async () => {
    set({ isLoading: true, error: null });
    try {
      const status = await api.getMarketStatus();
      set({ status, isOpen: status.is_open, isLoading: false });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to fetch market status',
        isLoading: false,
      });
    }
  },

  startPolling: () => {
    const { fetchStatus } = useMarketStore.getState();
    fetchStatus();
    const interval = setInterval(fetchStatus, 60_000);
    return () => clearInterval(interval);
  },
}));

// ─── Favorites Store ───

interface FavoritesState {
  favorites: InstrumentListItem[];
  favoriteIds: Set<number>;
  pendingIds: Set<number>;
  hasChanges: boolean;
  isLoading: boolean;
  isSaving: boolean;
  error: string | null;
  loadFavorites: () => Promise<void>;
  toggleFavorite: (instrumentId: number) => void;
  saveFavorites: () => Promise<void>;
  resetPending: () => void;
}

export const useFavoritesStore = create<FavoritesState>()((set, get) => ({
  favorites: [],
  favoriteIds: new Set<number>(),
  pendingIds: new Set<number>(),
  hasChanges: false,
  isLoading: false,
  isSaving: false,
  error: null,

  loadFavorites: async () => {
    set({ isLoading: true, error: null });
    try {
      const favorites = await api.getFavorites();
      const ids = new Set(favorites.map((f) => f.id));
      set({
        favorites,
        favoriteIds: ids,
        pendingIds: new Set(ids),
        hasChanges: false,
        isLoading: false,
      });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to load favorites',
        isLoading: false,
      });
    }
  },

  toggleFavorite: (instrumentId: number) => {
    const { pendingIds, favoriteIds } = get();
    const next = new Set(pendingIds);
    if (next.has(instrumentId)) {
      next.delete(instrumentId);
    } else {
      next.add(instrumentId);
    }
    const changed =
      next.size !== favoriteIds.size ||
      [...next].some((id) => !favoriteIds.has(id));
    set({ pendingIds: next, hasChanges: changed });
  },

  saveFavorites: async () => {
    const { pendingIds } = get();
    set({ isSaving: true, error: null });
    try {
      await api.updateFavorites([...pendingIds]);
      const favorites = await api.getFavorites();
      const ids = new Set(favorites.map((f) => f.id));
      set({
        favorites,
        favoriteIds: ids,
        pendingIds: new Set(ids),
        hasChanges: false,
        isSaving: false,
      });
    } catch (err) {
      set({
        error: err instanceof Error ? err.message : 'Failed to save favorites',
        isSaving: false,
      });
    }
  },

  resetPending: () => {
    const { favoriteIds } = get();
    set({ pendingIds: new Set(favoriteIds), hasChanges: false });
  },
}));
