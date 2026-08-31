import * as React from 'react';
import renderer from 'react-test-renderer';

// Mock useColorScheme to avoid react-native dependency issues in CI
jest.mock('../useColorScheme', () => ({
  useColorScheme: () => 'light',
}));

import { MonoText } from '../StyledText';

it(`renders correctly`, () => {
  const tree = renderer.create(<MonoText>Snapshot test!</MonoText>).toJSON();
  expect(tree).toMatchSnapshot();
});
