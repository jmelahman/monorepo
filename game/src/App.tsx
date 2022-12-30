import { Routes, Route } from "react-router-dom";
import { Provider } from 'mobx-react';

import CastlePage from './Castle';
import { resources } from './Store';
import './App.css';

export default function App() {
  return (
    <div className="App">
      <Provider resources={resources}>
        <Routes>
          <Route path="*" element={<CastlePage />}/>
        </Routes>
      </Provider>
    </div>
  );
}