import * as React from 'react';
import { useHistory } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const ScientistCreatePage: React.FC = () => {
  const history = useHistory();
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [field, setField] = React.useState('');
  const [name, setName] = React.useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {};
    if (field) body['field'] = field;
    if (name) body['name'] = name;

    try {
      const api = createAPIClient();
      const created = await api.scientists.create(body);
      history.push(`/trex-console/scientists/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/scientists')}
            component="button"
          >
            Scientists
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Scientist</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup
            label="Field"
            fieldId="field-field"
            isRequired={ true}
          >
            <TextInput
              id="field-field"
              type="text"
              value={ field}
              onChange={(_e, val) => setField(val)}
              placeholder="Enter Field"
              isRequired={ true}
            />
          </FormGroup>
          <FormGroup
            label="Name"
            fieldId="field-name"
            isRequired={ true}
          >
            <TextInput
              id="field-name"
              type="text"
              value={ name}
              onChange={(_e, val) => setName(val)}
              placeholder="Enter Name"
              isRequired={ true}
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/scientists')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default ScientistCreatePage;
